package sanitize

import (
	"context"
	"testing"

	"github.com/derailed/popeye/internal/cache"
	"github.com/derailed/popeye/internal/issues"
	"github.com/derailed/popeye/pkg/config"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	mv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

func TestDSSanitize(t *testing.T) {
	uu := map[string]struct {
		lister DaemonSetLister
		issues issues.Issues
	}{
		"good": {
			lister: makeDSLister("d1", dsOpts{
				coOpts: coOpts{
					image: "fred:0.0.1",
					rcpu:  "10m",
					rmem:  "10Mi",
					lcpu:  "10m",
					lmem:  "10Mi",
				},
				ccpu: "10m",
				cmem: "10Mi",
			}),
			issues: issues.Issues{},
		},
	}

	for k, u := range uu {
		t.Run(k, func(t *testing.T) {
			ds := NewDaemonSet(issues.NewCollector(loadCodes(t)), u.lister)
			ds.Sanitize(context.Background())

			assert.Equal(t, u.issues, ds.Outcome()["default/d1"])
		})
	}
}

func TestDSSanitizeUtilization(t *testing.T) {
	uu := map[string]struct {
		lister DaemonSetLister
		issues issues.Issues
	}{
		"bestEffort": {
			lister: makeDSLister("d1", dsOpts{
				coOpts: coOpts{
					image: "fred:0.0.1",
				},
				ccpu: "10m",
				cmem: "10Mi",
			}),
			issues: issues.Issues{
				issues.New("i1", issues.WarnLevel, "[POP-106] No resources defined"),
				issues.New("c1", issues.WarnLevel, "[POP-106] No resources defined"),
			},
		},
		"cpuUnderBurstable": {
			lister: makeDSLister("d1", dsOpts{
				coOpts: coOpts{
					image: "fred:0.0.1",
					rcpu:  "5m",
					rmem:  "10Mi",
					lcpu:  "10m",
					lmem:  "10Mi",
				},
				ccpu: "10m",
				cmem: "10Mi",
			}),
			issues: issues.Issues{
				issues.New(issues.Root, issues.WarnLevel, "[POP-503] At current load, CPU under allocated. Current:20m vs Requested:10m (200.00%)"),
			},
		},
		"cpuUnderGuaranteed": {
			lister: makeDSLister("d1", dsOpts{
				coOpts: coOpts{
					image: "fred:0.0.1",
					rcpu:  "5m",
					rmem:  "10Mi",
					lcpu:  "5m",
					lmem:  "10Mi",
				},
				ccpu: "10m",
				cmem: "10Mi",
			}),
			issues: issues.Issues{
				issues.New(issues.Root, issues.WarnLevel, "[POP-503] At current load, CPU under allocated. Current:20m vs Requested:10m (200.00%)"),
			},
		},
		// c=20 r=60 20/60=1/3 over is 50% req=3*c 33 > 100
		// c=60 r=20 60/20 3 under
		"cpuOverBustable": {
			lister: makeDSLister("d1", dsOpts{
				coOpts: coOpts{
					image: "fred:0.0.1",
					rcpu:  "30m",
					rmem:  "10Mi",
					lcpu:  "50m",
					lmem:  "10Mi",
				},
				ccpu: "10m",
				cmem: "10Mi",
			}),
			issues: issues.Issues{
				issues.New(issues.Root, issues.WarnLevel, "[POP-504] At current load, CPU over allocated. Current:20m vs Requested:60m (300.00%)"),
			},
		},
		"cpuOverGuaranteed": {
			lister: makeDSLister("d1", dsOpts{
				coOpts: coOpts{
					image: "fred:0.0.1",
					rcpu:  "30m",
					rmem:  "10Mi",
					lcpu:  "30m",
					lmem:  "10Mi",
				},
				ccpu: "10m",
				cmem: "10Mi",
			}),
			issues: issues.Issues{
				issues.New(issues.Root, issues.WarnLevel, "[POP-504] At current load, CPU over allocated. Current:20m vs Requested:60m (300.00%)"),
			},
		},
		"memUnderBurstable": {
			lister: makeDSLister("d1", dsOpts{
				coOpts: coOpts{
					image: "fred:0.0.1",
					rcpu:  "10m",
					rmem:  "5Mi",
					lcpu:  "20m",
					lmem:  "20Mi",
				},
				ccpu: "10m",
				cmem: "10Mi",
			}),
			issues: issues.Issues{
				issues.New(issues.Root, issues.WarnLevel, "[POP-505] At current load, Memory under allocated. Current:20Mi vs Requested:10Mi (200.00%)"),
			},
		},
		"memUnderGuaranteed": {
			lister: makeDSLister("d1", dsOpts{
				coOpts: coOpts{
					image: "fred:0.0.1",
					rcpu:  "10m",
					rmem:  "5Mi",
					lcpu:  "10m",
					lmem:  "5Mi",
				},
				ccpu: "10m",
				cmem: "10Mi",
			}),
			issues: issues.Issues{
				issues.New(issues.Root, issues.WarnLevel, "[POP-505] At current load, Memory under allocated. Current:20Mi vs Requested:10Mi (200.00%)"),
			},
		},
		"memOverBurstable": {
			lister: makeDSLister("d1", dsOpts{
				coOpts: coOpts{
					image: "fred:0.0.1",
					rcpu:  "10m",
					rmem:  "30Mi",
					lcpu:  "20m",
					lmem:  "60Mi",
				},
				ccpu: "10m",
				cmem: "10Mi",
			}),
			issues: issues.Issues{
				issues.New(issues.Root, issues.WarnLevel, "[POP-506] At current load, Memory over allocated. Current:20Mi vs Requested:60Mi (300.00%)"),
			},
		},
		"memOverGuaranteed": {
			lister: makeDSLister("d1", dsOpts{
				coOpts: coOpts{
					image: "fred:0.0.1",
					rcpu:  "10m",
					rmem:  "30Mi",
					lcpu:  "10m",
					lmem:  "30Mi",
				},
				ccpu: "10m",
				cmem: "10Mi",
			}),
			issues: issues.Issues{
				issues.New(issues.Root, issues.WarnLevel, "[POP-506] At current load, Memory over allocated. Current:20Mi vs Requested:60Mi (300.00%)"),
			},
		},
	}

	ctx := context.WithValue(context.Background(), PopeyeKey("OverAllocs"), true)
	for k, u := range uu {
		t.Run(k, func(t *testing.T) {
			ds := NewDaemonSet(issues.NewCollector(loadCodes(t)), u.lister)
			ds.Sanitize(ctx)

			assert.Equal(t, u.issues, ds.Outcome()["default/d1"])
		})
	}
}

// ----------------------------------------------------------------------------
// Helpers...

type (
	dsOpts struct {
		coOpts
		ccpu, cmem string
	}

	ds struct {
		name string
		opts dsOpts
	}
)

func makeDSLister(n string, opts dsOpts) *ds {
	return &ds{
		name: n,
		opts: opts,
	}
}

func (d *ds) CPUResourceLimits() config.Allocations {
	return config.Allocations{
		UnderPerc: 100,
		OverPerc:  50,
	}
}

func (d *ds) MEMResourceLimits() config.Allocations {
	return config.Allocations{
		UnderPerc: 100,
		OverPerc:  50,
	}
}

func (d *ds) ListPodsBySelector(sel *metav1.LabelSelector) map[string]*v1.Pod {
	return map[string]*v1.Pod{
		"default/p1": makeFullPod("p1", podOpts{
			coOpts: d.opts.coOpts,
		}),
	}
}

func (d *ds) RestartsLimit() int {
	return 10
}

func (d *ds) PodCPULimit() float64 {
	return 100
}

func (d *ds) PodMEMLimit() float64 {
	return 100
}

func (d *ds) ListPodsMetrics() map[string]*mv1beta1.PodMetrics {
	return map[string]*mv1beta1.PodMetrics{
		cache.FQN("default", "p1"): makeMxPod("p1", d.opts.ccpu, d.opts.cmem),
	}
}

func (d *ds) ListDaemonSets() map[string]*appsv1.DaemonSet {
	return map[string]*appsv1.DaemonSet{
		cache.FQN("default", d.name): makeDS(d.name, d.opts),
	}
}

func makeDS(n string, o dsOpts) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      n,
			Namespace: "default",
			SelfLink:  "/api/apps/v1/blee/blah",
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"fred": "blee",
				},
			},
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					InitContainers: []v1.Container{
						makeContainer("i1", o.coOpts),
					},
					Containers: []v1.Container{
						makeContainer("c1", o.coOpts),
					},
				},
			},
		},
		Status: appsv1.DaemonSetStatus{},
	}
}
