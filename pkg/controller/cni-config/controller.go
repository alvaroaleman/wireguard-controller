package cni_config

import (
	"net"
	"time"

	"github.com/mrincompetent/wireguard-controller/pkg/kubernetes"
	"github.com/vishvananda/netlink"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

const (
	name = "cni_config_controller"
)

func Add(
	mgr ctrl.Manager,
	log *zap.Logger,
	nodePodCidrNet, podCidrNet *net.IPNet,
	cniTemplateDir string,
	cniConfigPath string,
	interfaceName string,
) error {

	options := controller.Options{
		MaxConcurrentReconciles: 1,
		Reconciler: &Reconciler{
			log:            log.Named(name),
			nodePodCidrNet: nodePodCidrNet,
			podCidrNet:     podCidrNet,
			interfaceName:  interfaceName,
			cni: CNIConfig{
				TargetDir:   cniConfigPath,
				TemplateDir: cniTemplateDir,
			},
		},
	}
	c, err := controller.New(name, mgr, options)
	if err != nil {
		return err
	}

	return c.Watch(kubernetes.NewTickerSource(5*time.Second), &handler.EnqueueRequestForObject{})
}

type CNIConfig struct {
	TemplateDir string
	TargetDir   string
}

type Reconciler struct {
	log                        *zap.Logger
	nodePodCidrNet, podCidrNet *net.IPNet
	cni                        CNIConfig
	interfaceName              string
}

func (r *Reconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log := r.log.With()
	log.Debug("Processing")

	link, err := netlink.LinkByName(r.interfaceName)
	if err != nil {
		// In case the interface was not created yet we requeue
		if _, isNotFound := err.(netlink.LinkNotFoundError); isNotFound {
			log.Debug("Skipping route reconciling since the link is not up yet")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, errors.Wrap(err, "unable to get interface details")
	}

	if err := r.writeCNIConfig(log, link.Attrs().MTU); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "unable to write CNI config")
	}

	return ctrl.Result{}, nil
}
