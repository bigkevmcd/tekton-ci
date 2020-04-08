package cmd

import (
	"fmt"
	"log"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/jenkins-x/go-scm/scm/factory"
	pipelineclientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"

	"github.com/bigkevmcd/tekton-ci/pkg/git"
	"github.com/bigkevmcd/tekton-ci/pkg/githooks"
	"github.com/bigkevmcd/tekton-ci/pkg/githooks/dsl"
	"github.com/bigkevmcd/tekton-ci/pkg/githooks/pipelinerun"
	"github.com/bigkevmcd/tekton-ci/pkg/volumes"
)

func makeHTTPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "http",
		Short: "execute PipelineRuns in response to hooks",
		Run: func(cmd *cobra.Command, args []string) {
			scmClient, err := factory.NewClient(viper.GetString("driver"), "", "")
			if err != nil {
				log.Fatal(err)
			}

			clusterConfig, err := rest.InClusterConfig()
			if err != nil {
				log.Fatalf("failed to get in cluster config: %v", err)
			}

			tektonClient, err := pipelineclientset.NewForConfig(clusterConfig)
			if err != nil {
				log.Fatalf("failed to get the versioned client: %v", err)
			}

			coreClient, err := kubernetes.NewForConfig(clusterConfig)
			if err != nil {
				log.Fatalf("failed to get the versioned client: %v", err)
			}

			logger, _ := zap.NewProduction()
			defer logger.Sync() // flushes buffer, if any
			sugar := logger.Sugar()
			namespace := viper.GetString("namespace")

			dslHandler := dsl.New(
				git.New(scmClient),
				tektonClient,
				volumes.New(coreClient),
				namespace,
				sugar)
			pipelinerunHandler := pipelinerun.New(
				git.New(scmClient),
				tektonClient,
				namespace,
				sugar)
			http.Handle("/pipeline", githooks.New(
				git.New(scmClient),
				dslHandler,
				sugar,
			))
			http.Handle("/pipelinerun", githooks.New(
				git.New(scmClient),
				pipelinerunHandler,
				sugar,
			))
			listen := fmt.Sprintf(":%d", viper.GetInt("port"))
			log.Fatal(http.ListenAndServe(listen, nil))
		},
	}

	cmd.Flags().Int(
		"port",
		8080,
		"port to serve requests on",
	)
	logIfError(viper.BindPFlag("port", cmd.Flags().Lookup("port")))

	cmd.Flags().String(
		"driver",
		"github",
		"go-scm driver name to use e.g. github, gitlab",
	)
	logIfError(viper.BindPFlag("driver", cmd.Flags().Lookup("driver")))

	cmd.Flags().String(
		"namespace",
		"default",
		"namespace to execute PipelineRuns in",
	)
	logIfError(viper.BindPFlag("namespace", cmd.Flags().Lookup("namespace")))
	return cmd
}
