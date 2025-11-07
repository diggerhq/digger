package service_clients

import (
	"context"
	"fmt"
	"os"
	"time"

	"log/slog"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func newInClusterClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

type  K8sJobClient struct {
		clientset *kubernetes.Clientset
		namespace string
}


type JobOptions struct {
	// Identity
	NamePrefix         string            // e.g. "projects-refresh"
	ContainerName      string            // e.g. "projects-refresh"
	Image              string            // container image (fallback to k.image if empty)
	Labels             map[string]string // extra labels to place on Job/Pod
	Annotations        map[string]string // annotations on Pod template

	// Runtime / Pod
	ServiceAccountName string
	Command            []string
	Args               []string
	Env                map[string]string // simple key/val envs
	EnvVars            []corev1.EnvVar   // advanced envs (e.g. SecretKeyRef), appended after Env
	Volumes            []corev1.Volume
	VolumeMounts       []corev1.VolumeMount
	NodeSelector       map[string]string
	Tolerations        []corev1.Toleration

	// Resources (defaults if empty)
	CPU    string // e.g. "1", "500m"
	Memory string // e.g. "256Mi"

	// Job policy (defaults if zero)
	BackoffLimit          *int32
	TTLSecondsAfterFinish *int32
	ActiveDeadlineSeconds *int64
	RestartPolicy         corev1.RestartPolicy
}

func (k K8sJobClient) triggerJob(ctx context.Context, opt JobOptions) (*BackgroundJobTriggerResponse, error) {
	// Defaults
	if opt.NamePrefix == "" {
		opt.NamePrefix = "job"
	}
	if opt.ContainerName == "" {
		opt.ContainerName = opt.NamePrefix
	}
	if opt.Image == "" {
		return nil, fmt.Errorf("image must be provided (no default set on client)")
	}

	if opt.CPU == "" {
		opt.CPU = "1"
	}
	if opt.Memory == "" {
		opt.Memory = "256Mi"
	}
	if opt.RestartPolicy == "" {
		opt.RestartPolicy = corev1.RestartPolicyNever
	}
	// Reasonable defaults
	if opt.BackoffLimit == nil {
		v := int32(0)
		opt.BackoffLimit = &v
	}
	if opt.TTLSecondsAfterFinish == nil {
		v := int32(300)
		opt.TTLSecondsAfterFinish = &v
	}
	if opt.ActiveDeadlineSeconds == nil {
		v := int64(30 * 60) // 30 minutes
		opt.ActiveDeadlineSeconds = &v
	}

	// Name with timestamp for uniqueness
	name := fmt.Sprintf("%s-%d", opt.NamePrefix, time.Now().UnixMilli())

	// Resources
	cpuQty, err := resource.ParseQuantity(opt.CPU)
	if err != nil {
		return nil, fmt.Errorf("invalid CPU quantity %q: %w", opt.CPU, err)
	}
	memQty, err := resource.ParseQuantity(opt.Memory)
	if err != nil {
		return nil, fmt.Errorf("invalid Memory quantity %q: %w", opt.Memory, err)
	}

	// Build env vars
	env := make([]corev1.EnvVar, 0, len(opt.Env)+len(opt.EnvVars))
	for k, v := range opt.Env {
		env = append(env, corev1.EnvVar{Name: k, Value: v})
	}
	env = append(env, opt.EnvVars...) // allow SecretKeyRef, DownwardAPI, etc.

	// Base labels
	labels := map[string]string{
		"app.kubernetes.io/name":       opt.NamePrefix,
		"app.kubernetes.io/managed-by": "digger-jobs",
	}
	for k, v := range opt.Labels {
		labels[k] = v
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: k.namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            opt.BackoffLimit,
			TTLSecondsAfterFinished: opt.TTLSecondsAfterFinish, // requires TTL controller enabled
			ActiveDeadlineSeconds:   opt.ActiveDeadlineSeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: opt.Annotations,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: opt.ServiceAccountName,
					RestartPolicy:      opt.RestartPolicy,
					NodeSelector:       opt.NodeSelector,
					Tolerations:        opt.Tolerations,
					Volumes:            opt.Volumes,
					Containers: []corev1.Container{
						{
							Name:         opt.ContainerName,
							Image:        opt.Image,
							Command:      opt.Command,
							Args:         opt.Args,
							Env:          env,
							VolumeMounts: opt.VolumeMounts,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    cpuQty,
									corev1.ResourceMemory: memQty,
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    cpuQty,
									corev1.ResourceMemory: memQty,
								},
							},
						},
					},
				},
			},
		},
	}

	slog.Debug("creating k8s job", "namespace", k.namespace, "name", name)

	// Short, bounded context unless caller provided one
	if ctx == nil {
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ctx = c
	}

	created, err := k.clientset.BatchV1().Jobs(k.namespace).Create(job)
	if err != nil {
		slog.Error("error creating k8s job", "error", err)
		return nil, err
	}

	slog.Debug("triggered k8s job", "job", created.Name, "uid", created.UID)
	return &BackgroundJobTriggerResponse{ID: string(created.UID)}, nil
}

func (k K8sJobClient) TriggerProjectsRefreshService(
	cloneUrl, branch, githubToken, repoFullName, orgId string,
) (*BackgroundJobTriggerResponse, error) {
	image := "ghcr.io/diggerhq/digger/projects-refresh-service:v1.0.0"
	if projectsServiceImage := os.Getenv("PROJECTS_REFRESH_SERVICE_DOCKER_IMAGE"); projectsServiceImage != "" {
		image = projectsServiceImage
	}
	return k.triggerJob(context.Background(), JobOptions{
		NamePrefix:         "projects-refresh",
		ContainerName:      "projects-refresh",
		Image:              image,
		ServiceAccountName: "projects-refresh-sa",
		Labels: map[string]string{
			"app":           "projects-refresh-service",
			"orgId":         orgId,
			"repoFullName":  repoFullName,
			"triggerSource": "orchestrator-api",
		},
		Env: map[string]string{
			"CloneUrl":     cloneUrl,
			"Branch":       branch,
			"GithubToken":  githubToken, // consider moving to SecretKeyRef in production
			"RepoFullName": repoFullName,
			"OrgId":        orgId,
		},
		// Optionally override defaults:
		CPU:    "1",
		Memory: "512Mi",
		// BackoffLimit:          pointer.To(int32(0)),
		// TTLSecondsAfterFinish: pointer.To(int32(300)),
		// ActiveDeadlineSeconds: pointer.To(int64(1800)),
	})
}
