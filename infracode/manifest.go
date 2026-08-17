package infracode

import (
	"embed"
	"io/fs"
	"sort"
	"strings"

	homeenv "github.com/devilmonastery/env-k8s-home/home"
	"github.com/devilmonastery/infracode/contracts/delivery"
	"github.com/devilmonastery/infracode/contracts/workload"
	"github.com/devilmonastery/infracode/core/diag"
	"github.com/devilmonastery/infracode/core/engine"
	"github.com/devilmonastery/infracode/core/output"
	"github.com/devilmonastery/infracode/core/products"
	"github.com/devilmonastery/infracode/domains/cicd/drone"
	"github.com/devilmonastery/infracode/domains/dev/tilt"
	"github.com/devilmonastery/infracode/domains/kubernetes/manifestbundle"
	"github.com/devilmonastery/infracode/domains/makefile"
	"github.com/devilmonastery/infracode/domains/release"
	"github.com/devilmonastery/infracode/domains/renovate"
	"github.com/devilmonastery/infracode/infragen"
)

//go:embed source/*.yaml
var sourceFiles embed.FS

// Generate is the infracode manifest entrypoint for Hivemind.
func Generate(gen *infragen.Generator) {
	home := homeenv.New(gen, "home")
	dev := home.Development("dev")
	prod := home.Production("prod",
		homeenv.DefaultDelivery(delivery.Argo(
			delivery.WithRepoURL("https://github.com/devilmonastery/hivemind.git"),
			delivery.WithDocsLink("https://github.com/devilmonastery/hivemind/blob/main/README.md"),
			delivery.WithRepositoryLink("https://github.com/devilmonastery/hivemind"),
		)),
	)

	manifestbundle.New(gen,
		manifestbundle.Named("hivemind"),
		manifestbundle.WithOutputPath(".infracode/environments/home/prod/kubernetes/hivemind"),
		manifestbundle.WithContents(sourceContents()),
	)

	workloadDomain := &bundleWorkloadDomain{}
	gen.Register(workloadDomain)
	release.New(gen,
		release.In(prod),
		release.DeliveredBy(prod.ApplyDeliveryPolicy(delivery.Argo(
			delivery.WithRepoURL("https://github.com/devilmonastery/hivemind.git"),
			delivery.WithDocsLink("https://github.com/devilmonastery/hivemind/blob/main/README.md"),
			delivery.WithRepositoryLink("https://github.com/devilmonastery/hivemind"),
		))),
		release.Of(workloadRef{}),
	)
	drone.New(gen,
		drone.WithGoModuleAuth("github.com/devilmonastery/*", "github.com/devilmonastery/*", "github_module_token"),
	)
	tilt.DevLoop(gen,
		tilt.In(dev),
		tilt.WithDevNamespace("tilt-dev"),
		tilt.WithWorkloads(workloadRef{}),
	)
	makefile.New(gen)
	renovate.New(gen)
}

func sourceContents() string {
	entries, err := fs.Glob(sourceFiles, "source/*.yaml")
	if err != nil {
		panic(err)
	}
	sort.Strings(entries)
	var documents []string
	for _, name := range entries {
		body, err := sourceFiles.ReadFile(name)
		if err != nil {
			panic(err)
		}
		documents = append(documents, strings.TrimSpace(string(body)))
	}
	return strings.Join(documents, "\n---\n")
}

type workloadRef struct{}

func (workloadRef) Ref() workload.Ref { return workload.NewRef("hivemind") }

type bundleWorkloadDomain struct{}

func (*bundleWorkloadDomain) Name() string { return "hivemind.workload" }

func (*bundleWorkloadDomain) Define(*engine.Context) error { return nil }

func (*bundleWorkloadDomain) Resolve(ctx *engine.Context) error {
	return products.Put(ctx.Products, workload.NewRef("hivemind").ProductKey(), workload.Product{
		Ref:               workload.NewRef("hivemind"),
		Name:              "hivemind",
		Namespace:         "hivemind",
		Image:             "registry.local.rothwell.us/hivemind-server",
		Ports:             []workload.Port{{Name: "grpc", Port: 4153}, {Name: "metrics", Port: 4163}},
		RuntimeOutputPath: ".infracode/environments/home/prod/kubernetes/hivemind",
	})
}

func (*bundleWorkloadDomain) Validate(*engine.Context, *diag.Collector) error { return nil }

func (*bundleWorkloadDomain) Render(*engine.Context, output.Writer) error { return nil }
