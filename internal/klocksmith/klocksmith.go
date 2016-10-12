package klocksmith

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/coreos/go-systemd/login1"
	"k8s.io/client-go/1.4/kubernetes"
	v1core "k8s.io/client-go/1.4/kubernetes/typed/core/v1"
	"k8s.io/client-go/1.4/pkg/api"
	v1api "k8s.io/client-go/1.4/pkg/api/v1"
	"k8s.io/client-go/1.4/pkg/fields"
	"k8s.io/client-go/1.4/pkg/watch"

	"github.com/coreos-inc/klocksmith/internal/drain"
	"github.com/coreos-inc/klocksmith/internal/k8sutil"
	"github.com/coreos-inc/klocksmith/internal/updateengine"
)

type Klocksmith struct {
	node string
	kc   *kubernetes.Clientset
	nc   v1core.NodeInterface
	ue   *updateengine.Client
	lc   *login1.Conn
}

func New(node string) (*Klocksmith, error) {
	// set up kubernetes in-cluster client
	kc, err := k8s()
	if err != nil {
		return nil, fmt.Errorf("error creating kubernetes client: %v", err)
	}

	// node interface
	nc := kc.Nodes()

	// set up update_engine client
	ue, err := updateengine.New()
	if err != nil {
		return nil, fmt.Errorf("error establishing connection to update_engine dbus: %v", err)
	}

	// set up login1 client for our eventual reboot
	lc, err := login1.New()
	if err != nil {
		return nil, fmt.Errorf("error establishing connection to logind dbus: %v", err)
	}

	return &Klocksmith{node, kc, nc, ue, lc}, nil
}

// Run runs klocksmithd, reacting to the update_engine reboot signal by
// draining pods on this kubernetes node and rebooting.
//
// TODO: try to be more resilient against transient failures
func (k *Klocksmith) Run() error {
	// set coreos.com/update1/reboot-in-progress=false and
	// coreos.com/update1/reboot-needed=false
	anno := map[string]string{
		labelRebootInProgress: "false",
		labelRebootNeeded:     "false",
	}
	log.Printf("Setting labels %#v", anno)
	if err := setNodeLabels(k.nc, k.node, anno); err != nil {
		return err
	}

	// we are schedulable now.
	log.Print("Marking node as schedulable")
	if err := unschedulable(k.nc, k.node, false); err != nil {
		return err
	}

	// block until update_engine says to reboot, and set label.
	log.Printf("Waiting for reboot signal...")
	if err := k.waitForRebootSignal(); err != nil {
		return err
	}

	// indicate we need a reboot
	anno = map[string]string{
		labelRebootNeeded: "true",
	}
	log.Printf("Setting labels %#v", anno)
	if err := setNodeLabels(k.nc, k.node, anno); err != nil {
		return err
	}

	// block until labelOkToReboot is set
	log.Printf("Waiting for ok-to-reboot from controller...")
	if err := k.waitForOkToReboot(); err != nil {
		return err
	}

	// set labelRebootInProgress and drain self
	anno = map[string]string{
		labelRebootInProgress: "true",
	}
	log.Printf("Setting labels %#v", anno)
	if err := setNodeLabels(k.nc, k.node, anno); err != nil {
		return err
	}

	// drain self equates to:
	// 1. set Unschedulable
	// 2. delete all pods
	// unlike `kubectl drain`, we do not care about emptyDir or orphan pods
	// ('any pods that are neither mirror pods nor managed by
	// ReplicationController, ReplicaSet, DaemonSet or Job')

	log.Print("Marking node as unschedulable")
	if err := unschedulable(k.nc, k.node, true); err != nil {
		return err
	}

	log.Print("Getting pod list for deletion")
	pods, err := k.getPodsForDeletion()
	if err != nil {
		return err
	}

	// delete the pods.
	// TODO: explicitly don't terminate self? we'll probably just be a
	// mirror pod or daemonset anyway..
	log.Printf("Deleting %d pods", len(pods))
	deleteOptions := api.NewDeleteOptions(30)
	for _, pod := range pods {
		log.Printf("Terminating pod %q...", pod.Name)
		if err := k.kc.Pods(pod.Namespace).Delete(pod.Name, deleteOptions); err != nil {
			return fmt.Errorf("failed terminating pod %q: %v", pod.Name, err)
		}
	}

	log.Print("Node drained, rebooting")

	// reboot
	k.lc.Reboot(false)

	// cross fingers
	time.Sleep(24 * 7 * time.Hour)

	return nil
}

// waitForRebootSignal watches update_engine and waits for
// updateengine.UpdateStatusUpdatedNeedReboot
func (k *Klocksmith) waitForRebootSignal() error {
	// first, check current status.
	status, err := k.ue.GetStatus()
	if err != nil {
		return fmt.Errorf("failed to get update_engine status: %v", err)
	}

	// if we're not in UpdateStatusUpdatedNeedReboot, then wait for it.
	if status.CurrentOperation != updateengine.UpdateStatusUpdatedNeedReboot {
		ctx, cancel := context.WithCancel(context.Background())
		stc := make(chan updateengine.Status)
		go k.ue.RebootNeededSignal(stc, ctx.Done())
		<-stc
		cancel()
	}

	// we are now in UpdateStatusUpdatedNeedReboot state.
	return nil
}

func (k *Klocksmith) waitForOkToReboot() error {
	n, err := k.nc.Get(k.node)
	if err != nil {
		return fmt.Errorf("failed to get self node (%q): %v", k.node, err)
	}

	// XXX: set timeout > 0?
	watcher, err := k.nc.Watch(api.ListOptions{
		FieldSelector:   fields.OneTermEqualSelector("metadata.name", n.Name),
		ResourceVersion: n.ResourceVersion,
	})
	if err != nil {
		return fmt.Errorf("failed to watch self node (%q): %v", k.node, err)
	}

	// hopefully 24 hours is enough time between indicating we need a
	// reboot and the controller telling us to do it
	ev, err := watch.Until(time.Hour*24, watcher, nodeLabelCondition(labelOkToReboot, "true"))
	if err != nil {
		return fmt.Errorf("waiting for label %q failed: %v", labelOkToReboot, err)
	}

	// sanity check
	no, ok := ev.Object.(*v1api.Node)
	if !ok {
		panic("event contains a non-*api.Node object")
	}

	if no.Labels[labelOkToReboot] != "true" {
		panic("event did not contain label expected")
	}

	return nil
}

func (k *Klocksmith) getPodsForDeletion() ([]v1api.Pod, error) {
	pods, err := drain.GetPodsForDeletion(k.kc, k.node)
	if err != nil {
		return nil, fmt.Errorf("failed to get list of pods for deletion: %v", err)
	}

	// XXX: ignoring kube-system is a simple way to avoid eviciting
	// critical components such as kube-scheduler and
	// kube-controller-manager.

	pods = k8sutil.FilterPods(pods, func(p *v1api.Pod) bool {
		if p.Namespace == "kube-system" {
			return false
		}

		return true
	})

	return pods, nil
}