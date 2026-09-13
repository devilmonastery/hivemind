package infracode

import (
	"embed"
	"io/fs"
	"sort"
	"strings"

	homeenv "github.com/devilmonastery/env-k8s-home/home"
	"github.com/devilmonastery/infracode/contracts/delivery"
	envcontract "github.com/devilmonastery/infracode/contracts/environment"
	storagecontract "github.com/devilmonastery/infracode/contracts/storage"
	"github.com/devilmonastery/infracode/contracts/workload"
	"github.com/devilmonastery/infracode/core/diag"
	"github.com/devilmonastery/infracode/core/engine"
	"github.com/devilmonastery/infracode/core/output"
	"github.com/devilmonastery/infracode/core/products"
	"github.com/devilmonastery/infracode/domains/argo"
	"github.com/devilmonastery/infracode/domains/cicd/drone"
	"github.com/devilmonastery/infracode/domains/dev/tilt"
	"github.com/devilmonastery/infracode/domains/kubernetes/manifestbundle"
	postgresdomain "github.com/devilmonastery/infracode/domains/kubernetes/postgres"
	"github.com/devilmonastery/infracode/domains/kubernetes/stack"
	k8sworkload "github.com/devilmonastery/infracode/domains/kubernetes/workload"
	"github.com/devilmonastery/infracode/domains/makefile"
	"github.com/devilmonastery/infracode/domains/renovate"
	"github.com/devilmonastery/infracode/infragen"
)

//go:embed source/*.yaml
var sourceFiles embed.FS

// Generate is the infracode manifest entrypoint for Hivemind.
func Generate(gen *infragen.Generator) {
	home := homeenv.New(gen, "home")
	homeenv.IncludeGuidance(gen)
	dev := home.Development("dev")
	prod := home.Production("prod",
		homeenv.DefaultDelivery(delivery.Argo(
			delivery.WithRepoURL("https://github.com/devilmonastery/hivemind.git"),
			delivery.WithDocsLink("https://github.com/devilmonastery/hivemind/blob/main/README.md"),
			delivery.WithRepositoryLink("https://github.com/devilmonastery/hivemind"),
		)),
	)
	homeenv.EnableEnvironmentRunbooks(gen, prod)

	manifestbundle.New(gen,
		manifestbundle.Named("hivemind"),
		manifestbundle.WithOutputPath(".infracode/environments/home/prod/kubernetes/hivemind"),
		manifestbundle.WithContents(sourceContents()),
		manifestbundle.WithResources(postgresResources(prod)...),
	)

	workloadDomain := &bundleWorkloadDomain{}
	gen.Register(workloadDomain)
	gen.PublishShadowRoute(argo.RouteApplications)
	argo.NewApplicationsDomain(gen, argo.Config{
		OutputPath: "environments/home/prod/argocd",
		Umbrella: argo.Application{
			Name:                 "hivemind",
			Namespace:            "argocd",
			Project:              "workloads",
			RepoURL:              "https://github.com/devilmonastery/hivemind.git",
			TargetRevision:       "HEAD",
			Path:                 ".infracode/environments/home/prod/kubernetes/hivemind",
			DestinationServer:    "https://kubernetes.default.svc",
			DestinationNamespace: "hivemind",
			SyncPolicy: argo.SyncPolicy{
				Automated: true,
				Prune:     true,
				SelfHeal:  true,
				SyncOptions: []string{
					"CreateNamespace=true",
					"PrunePropagationPolicy=foreground",
					"PruneLast=true",
					"ServerSideApply=true",
				},
			},
		},
	})
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
		if name == "source/postgres.yaml" {
			continue
		}
		body, err := sourceFiles.ReadFile(name)
		if err != nil {
			panic(err)
		}
		documents = append(documents, strings.TrimSpace(string(body)))
	}
	return strings.Join(documents, "\n---\n")
}

func postgresResources(prod envcontract.Product) []stack.Resource {
	resources, err := postgresdomain.Resources(postgresdomain.Config{
		Meta: k8sworkload.Meta{
			Name:      "postgres",
			Namespace: "hivemind",
			Annotations: map[string]string{
				"argocd.argoproj.io/sync-options": "Replace=true",
			},
		},
		Image: postgresdomain.ImageSettings{
			Reference: "registry.local.rothwell.us/postgres-gembed:18.6-bookworm-pgvector0.8.6-pggembed1.0.0-minilm-l6-v2-r1@sha256:e65b3e85519e8c749c6cef3a94a4801db8f9b7b8f7d053c20ee8c49f20065a16",
			PGData:    "/var/lib/postgresql/18/docker",
			Ownership: postgresdomain.OwnershipEntrypoint,
		},
		Database: postgresdomain.DatabaseSettings{
			Name:              "hivemind",
			User:              "postgres",
			PasswordSecret:    "postgres-secret",
			PasswordSecretKey: "db-password",
		},
		Volume: storagecontract.NewVolumeIntent(
			homeenv.DatabaseRWORef(prod), "postgres-pvc", "10Gi", "/var/lib/postgresql",
			storagecontract.WithPurpose(storagecontract.PurposeDatabase),
			storagecontract.WithRetentionPolicy(storagecontract.RetainIndefinitely),
		),
		Resources: &k8sworkload.ResourceRequirements{
			Requests: map[string]string{"cpu": "250m", "memory": "512Mi"},
			Limits:   map[string]string{"cpu": "1500m", "memory": "2Gi"},
		},
	})
	if err != nil {
		panic(err)
	}
	return resources
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
