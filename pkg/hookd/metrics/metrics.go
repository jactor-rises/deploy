package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/navikt/deployment/pkg/pb"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	namespace = "deployment"
	subsystem = "hookd"

	StatusOK    = "ok"
	StatusError = "error"

	LabelStatus          = "status"
	LabelStatusCode      = "status_code"
	LabelDeploymentState = "deployment_state"
	Repository           = "repository"
	Team                 = "team"
	Cluster              = "cluster"
)

var (
	deployQueue        = make(map[string]interface{})
	clusterConnections = make(map[string]bool)
)

func GitHubRequest(statusCode int, repository, team string) {
	githubRequests.With(prometheus.Labels{
		LabelStatusCode: strconv.Itoa(statusCode),
		Repository:      repository,
		Team:            team,
	}).Inc()
}

func SetConnectedClusters(clusters []string) {
	for k := range clusterConnections {
		clusterConnections[k] = false
	}
	for _, k := range clusters {
		clusterConnections[k] = true
	}
	for k := range clusterConnections {
		i := 0.0
		if clusterConnections[k] {
			i = 1.0
		}
		clusterStatus.With(prometheus.Labels{
			Cluster: k,
		}).Set(i)
	}
}

func statusLabel(err error) string {
	if err == nil {
		return StatusOK
	}
	return StatusError
}

func DatabaseQuery(t time.Time, err error) {
	elapsed := time.Since(t)
	databaseQueries.With(prometheus.Labels{
		LabelStatus: statusLabel(err),
	}).Observe(elapsed.Seconds())
}

func UpdateQueue(status pb.DeploymentStatus) {
	stateTransitions.With(prometheus.Labels{
		LabelDeploymentState: status.GetState().String(),
		Repository:           status.GetDeployment().GetRepository().FullName(),
		Team:                 status.GetTeam(),
		Cluster:              status.GetCluster(),
	}).Inc()

	switch status.GetState() {

	// These three states are definite and signify the end of a deployment.
	case pb.GithubDeploymentState_success:

		// In case of successful deployment, report the lead time.
		ttd := float64(time.Now().Sub(status.Timestamp()))
		leadTime.With(prometheus.Labels{
			LabelDeploymentState: status.GetState().String(),
			Repository:           status.GetDeployment().GetRepository().FullName(),
			Team:                 status.GetTeam(),
			Cluster:              status.GetCluster(),
		}).Observe(ttd)

		fallthrough
	case pb.GithubDeploymentState_error:
		fallthrough
	case pb.GithubDeploymentState_failure:
		delete(deployQueue, status.GetDeliveryID())

	// Other states mean the deployment is still being processed.
	default:
		deployQueue[status.GetDeliveryID()] = new(interface{})
	}

	queueSize.Set(float64(len(deployQueue)))
}

var (
	databaseQueries = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:      "database_queries",
		Help:      "time to execute database queries",
		Namespace: namespace,
		Subsystem: subsystem,
		Buckets:   prometheus.LinearBuckets(0.005, 0.005, 20),
	},
		[]string{
			LabelStatus,
		},
	)

	githubRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:      "github_requests",
		Help:      "number of Github requests made",
		Namespace: namespace,
		Subsystem: subsystem,
	},
		[]string{
			LabelStatusCode,
			Repository,
			Team,
		},
	)

	stateTransitions = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:      "state_transition",
		Help:      "deployment state transitions",
		Namespace: namespace,
		Subsystem: subsystem,
	},
		[]string{
			LabelDeploymentState,
			Repository,
			Team,
			Cluster,
		},
	)

	queueSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:      "queue_size",
		Help:      "number of unfinished deployments",
		Namespace: namespace,
		Subsystem: subsystem,
	})

	clusterStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "cluster_status",
		Help:      "0 if cluster is down, 1 if cluster is up",
		Namespace: namespace,
		Subsystem: subsystem,
	},
		[]string{
			Cluster,
		},
	)

	leadTime = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name:      "lead_time_seconds",
		Help:      "the time it takes from a deploy is made to it is running in the cluster",
		Namespace: namespace,
		Subsystem: subsystem,
	},
		[]string{
			LabelDeploymentState,
			Repository,
			Team,
			Cluster,
		},
	)
)

func init() {
	prometheus.MustRegister(databaseQueries)
	prometheus.MustRegister(githubRequests)
	prometheus.MustRegister(stateTransitions)
	prometheus.MustRegister(queueSize)
	prometheus.MustRegister(leadTime)
	prometheus.MustRegister(clusterStatus)
}

func Handler() http.Handler {
	return promhttp.Handler()
}
