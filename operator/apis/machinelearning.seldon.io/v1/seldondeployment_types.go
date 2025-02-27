/*
Copyright 2019 The Seldon Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"crypto/md5"
	"encoding/hex"
	"os"
	"strconv"

	"k8s.io/apimachinery/pkg/types"

	kedav1alpha1 "github.com/kedacore/keda/api/v1alpha1"
	"github.com/seldonio/seldon-core/operator/constants"
	autoscalingv2beta2 "k8s.io/api/autoscaling/v2beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	Label_seldon_id          = "seldon-deployment-id"
	Label_seldon_app         = "seldon-app"
	Label_seldon_app_svc     = "seldon-app-svc"
	Label_svc_orch           = "seldon-deployment-contains-svcorch"
	Label_app                = "app"
	Label_fluentd            = "fluentd"
	Label_router             = "seldon.io/router"
	Label_combiner           = "seldon.io/combiner"
	Label_model              = "seldon.io/model"
	Label_transformer        = "seldon.io/transformer"
	Label_output_transformer = "seldon.io/output-transformer"
	Label_shadow             = "seldon.io/shadow"
	Label_explainer          = "seldon.io/explainer"
	Label_managed_by         = "app.kubernetes.io/managed-by"
	Label_value_seldon       = "seldon-core"

	PODINFO_VOLUME_NAME     = "seldon-podinfo"
	OLD_PODINFO_VOLUME_NAME = "podinfo"
	PODINFO_VOLUME_PATH     = "/etc/podinfo"

	ENV_PREDICTIVE_UNIT_SERVICE_PORT         = "PREDICTIVE_UNIT_SERVICE_PORT"
	ENV_PREDICTIVE_UNIT_HTTP_SERVICE_PORT    = "PREDICTIVE_UNIT_HTTP_SERVICE_PORT"
	ENV_PREDICTIVE_UNIT_GRPC_SERVICE_PORT    = "PREDICTIVE_UNIT_GRPC_SERVICE_PORT"
	ENV_PREDICTIVE_UNIT_SERVICE_PORT_METRICS = "PREDICTIVE_UNIT_METRICS_SERVICE_PORT"
	ENV_PREDICTIVE_UNIT_METRICS_ENDPOINT     = "PREDICTIVE_UNIT_METRICS_ENDPOINT"
	ENV_PREDICTIVE_UNIT_METRICS_PORT_NAME    = "PREDICTIVE_UNIT_METRICS_PORT_NAME"
	ENV_PREDICTIVE_UNIT_PARAMETERS           = "PREDICTIVE_UNIT_PARAMETERS"
	ENV_PREDICTIVE_UNIT_IMAGE                = "PREDICTIVE_UNIT_IMAGE"
	ENV_PREDICTIVE_UNIT_ID                   = "PREDICTIVE_UNIT_ID"
	ENV_PREDICTOR_ID                         = "PREDICTOR_ID"
	ENV_PREDICTOR_LABELS                     = "PREDICTOR_LABELS"
	ENV_SELDON_DEPLOYMENT_ID                 = "SELDON_DEPLOYMENT_ID"
	ENV_SELDON_EXECUTOR_ENABLED              = "SELDON_EXECUTOR_ENABLED"
	ENV_DEPLOYMENT_NAME_AS_PREFIX            = "DEPLOYMENT_NAME_AS_PREFIX"

	ANNOTATION_SEPARATE_ENGINE         = "seldon.io/engine-separate-pod"
	ANNOTATION_HEADLESS_SVC            = "seldon.io/headless-svc"
	ANNOTATION_NO_ENGINE               = "seldon.io/no-engine"
	ANNOTATION_CUSTOM_SVC_NAME         = "seldon.io/svc-name"
	ANNOTATION_LOGGER_WORK_QUEUE_SIZE  = "seldon.io/executor-logger-queue-size"
	ANNOTATION_LOGGER_WRITE_TIMEOUT_MS = "seldon.io/executor-logger-write-timeout-ms"

	DeploymentNamePrefix = "seldon"
)

var (
	envDeploymentNameAsPrefix = os.Getenv(ENV_DEPLOYMENT_NAME_AS_PREFIX)
)

func hash(text string) string {
	hasher := md5.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}

func GetSeldonDeploymentName(mlDep *SeldonDeployment) string {
	name := mlDep.Name
	if len(name) > 63 {
		return "seldon-" + hash(name)
	} else {
		return name
	}
}

func GetExplainerDeploymentName(sdepName string, predictorSpec *PredictorSpec) string {
	name := sdepName + "-" + predictorSpec.Name + constants.ExplainerNameSuffix
	if len(name) > 63 {
		return "seldon-" + hash(name)
	} else {
		return name
	}
}

func getContainerNames(containers []v1.Container) string {
	name := ""
	for i, c := range containers {
		if i > 0 {
			name = name + "-"
		}
		name = name + c.Name
	}
	return name
}

func GetDeploymentName(mlDep *SeldonDeployment, predictorSpec PredictorSpec, podSpec *SeldonPodSpec, podSpecIdx int) string {
	baseName := mlDep.Name + "-" + predictorSpec.Name + "-" + strconv.Itoa(podSpecIdx) + "-"
	var name string
	if podSpec != nil && len(podSpec.Metadata.Name) != 0 {
		name = baseName + podSpec.Metadata.Name
	} else {
		name = baseName + getContainerNames(podSpec.Spec.Containers)
	}
	if len(name) > 63 {
		if envDeploymentNameAsPrefix == "true" {
			possibleName := mlDep.Name + "-" + hash(name)
			if len(possibleName) <= 63 { // Check that the created name is still less than k8s limit
				return possibleName
			}
		}
		return DeploymentNamePrefix + "-" + hash(name) // default name we know will be ok
	} else {
		return name
	}
}

func GetServiceOrchestratorName(mlDep *SeldonDeployment, p *PredictorSpec) string {
	svcOrchName := mlDep.Name + "-" + p.Name + "-svc-orch"
	if len(svcOrchName) > 63 {
		return "seldon-" + hash(svcOrchName)
	} else {
		return svcOrchName
	}
}

func GetPredictorKey(mlDep *SeldonDeployment, p *PredictorSpec) string {
	if annotation, hasAnnotation := p.Annotations[ANNOTATION_CUSTOM_SVC_NAME]; hasAnnotation {
		return annotation
	} else {
		return getPredictorKeyAutoGenerated(mlDep, p)
	}
}

func getPredictorKeyAutoGenerated(mlDep *SeldonDeployment, p *PredictorSpec) string {
	pName := mlDep.Name + "-" + p.Name
	if len(pName) > 63 {
		return "seldon-" + hash(pName)
	} else {
		return pName
	}
}

func GetPredictiveUnit(pu *PredictiveUnit, name string) *PredictiveUnit {
	if name == pu.Name {
		return pu
	} else {
		for i := 0; i < len(pu.Children); i++ {
			found := GetPredictiveUnit(&pu.Children[i], name)
			if found != nil {
				return found
			}
		}
		return nil
	}
}

// if engine is not separated then this tells us which pu it should go on, as the mutating webhook handler has set host as localhost on the pu
func GetEnginePredictiveUnit(pu *PredictiveUnit) *PredictiveUnit {
	if pu.Endpoint != nil && pu.Endpoint.ServiceHost == "localhost" {
		return pu
	} else {
		for i := 0; i < len(pu.Children); i++ {
			found := GetEnginePredictiveUnit(&pu.Children[i])
			if found != nil {
				return found
			}
		}
		return nil
	}
}

func GetPredictiveUnitList(p *PredictiveUnit) (list []*PredictiveUnit) {
	list = append(list, p)

	for i := 0; i < len(p.Children); i++ {
		pu := &p.Children[i]
		list = append(list, GetPredictiveUnitList(pu)...)
	}
	return list
}

func GetContainerServiceName(mlDepName string, predictorSpec PredictorSpec, c *v1.Container) string {
	svcName := mlDepName + "-" + predictorSpec.Name + "-" + c.Name
	if len(svcName) > 63 {
		return "seldon-" + hash(svcName)
	} else {
		return svcName
	}
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SeldonDeploymentSpec defines the desired state of SeldonDeployment
type SeldonDeploymentSpec struct {
	//Name is Deprecated will be removed in future
	Name        string            `json:"name,omitempty" protobuf:"string,1,opt,name=name"`
	Predictors  []PredictorSpec   `json:"predictors" protobuf:"bytes,2,opt,name=name"`
	OauthKey    string            `json:"oauth_key,omitempty" protobuf:"string,3,opt,name=oauth_key"`
	OauthSecret string            `json:"oauth_secret,omitempty" protobuf:"string,4,opt,name=oauth_secret"`
	Annotations map[string]string `json:"annotations,omitempty" protobuf:"bytes,5,opt,name=annotations"`
	Protocol    Protocol          `json:"protocol,omitempty" protobuf:"bytes,6,opt,name=protocol"`
	Transport   Transport         `json:"transport,omitempty" protobuf:"bytes,7,opt,name=transport"`
	Replicas    *int32            `json:"replicas,omitempty" protobuf:"bytes,8,opt,name=replicas"`
	ServerType  ServerType        `json:"serverType,omitempty" protobuf:"bytes,9,opt,name=serverType"`
}

type SSL struct {
	CertSecretName string `json:"certSecretName,omitempty" protobuf:"string,2,opt,name=certSecretName"`
}

type PredictorSpec struct {
	Name            string                  `json:"name" protobuf:"string,1,opt,name=name"`
	Graph           PredictiveUnit          `json:"graph" protobuf:"bytes,2,opt,name=predictiveUnit"`
	ComponentSpecs  []*SeldonPodSpec        `json:"componentSpecs,omitempty" protobuf:"bytes,3,opt,name=componentSpecs"`
	Replicas        *int32                  `json:"replicas,omitempty" protobuf:"string,4,opt,name=replicas"`
	Annotations     map[string]string       `json:"annotations,omitempty" protobuf:"bytes,5,opt,name=annotations"`
	EngineResources v1.ResourceRequirements `json:"engineResources,omitempty" protobuf:"bytes,6,opt,name=engineResources"`
	Labels          map[string]string       `json:"labels,omitempty" protobuf:"bytes,7,opt,name=labels"`
	SvcOrchSpec     SvcOrchSpec             `json:"svcOrchSpec,omitempty" protobuf:"bytes,8,opt,name=svcOrchSpec"`
	Traffic         int32                   `json:"traffic,omitempty" protobuf:"bytes,9,opt,name=traffic"`
	Explainer       *Explainer              `json:"explainer,omitempty" protobuf:"bytes,10,opt,name=explainer"`
	Shadow          bool                    `json:"shadow,omitempty" protobuf:"bytes,11,opt,name=shadow"`
	SSL             *SSL                    `json:"ssl,omitempty" protobuf:"bytes,12,opt,name=ssl"`
}

type Protocol string

const (
	ProtocolSeldon     Protocol = "seldon"
	ProtocolTensorflow Protocol = "tensorflow"
	ProtocolKfserving  Protocol = "kfserving"
)

type Transport string

const (
	TransportRest Transport = "rest"
	TransportGrpc Transport = "grpc"
)

type ServerType string

const (
	ServerRPC   ServerType = "rpc"
	ServerKafka ServerType = "kafka"
)

type SvcOrchSpec struct {
	Resources *v1.ResourceRequirements `json:"resources,omitempty" protobuf:"bytes,1,opt,name=resources"`
	Env       []*v1.EnvVar             `json:"env,omitempty" protobuf:"bytes,2,opt,name=env"`
	Replicas  *int32                   `json:"replicas,omitempty" protobuf:"bytes,3,opt,name=replicas"`
}

type AlibiExplainerType string

const (
	AlibiAnchorsTabularExplainer      AlibiExplainerType = "AnchorTabular"
	AlibiAnchorsImageExplainer        AlibiExplainerType = "AnchorImages"
	AlibiAnchorsTextExplainer         AlibiExplainerType = "AnchorText"
	AlibiCounterfactualsExplainer     AlibiExplainerType = "Counterfactuals"
	AlibiContrastiveExplainer         AlibiExplainerType = "Contrastive"
	AlibiKernelShapExplainer          AlibiExplainerType = "KernelShap"
	AlibiIntegratedGradientsExplainer AlibiExplainerType = "IntegratedGradients"
	AlibiALEExplainer                 AlibiExplainerType = "ALE"
	AlibiTreeShap                     AlibiExplainerType = "TreeShap"
)

type Explainer struct {
	Type                    AlibiExplainerType `json:"type,omitempty" protobuf:"string,1,opt,name=type"`
	ModelUri                string             `json:"modelUri,omitempty" protobuf:"string,2,opt,name=modelUri"`
	ServiceAccountName      string             `json:"serviceAccountName,omitempty" protobuf:"string,3,opt,name=serviceAccountName"`
	ContainerSpec           v1.Container       `json:"containerSpec,omitempty" protobuf:"bytes,4,opt,name=containerSpec"`
	Config                  map[string]string  `json:"config,omitempty" protobuf:"bytes,5,opt,name=config"`
	Endpoint                *Endpoint          `json:"endpoint,omitempty" protobuf:"bytes,6,opt,name=endpoint"`
	EnvSecretRefName        string             `json:"envSecretRefName,omitempty" protobuf:"bytes,7,opt,name=envSecretRefName"`
	StorageInitializerImage string             `json:"storageInitializerImage,omitempty" protobuf:"bytes,8,opt,name=storageInitializerImage"`
	Replicas                *int32             `json:"replicas,omitempty" protobuf:"string,9,opt,name=replicas"`
	InitParameters          string             `json:"initParameters,omitempty" protobuf:"string,10,opt,name=initParameters"`
}

// ObjectMeta is a copy of the "k8s.io/apimachinery/pkg/apis/meta/v1" ObjectMeta.
// We copy it for 2 reasons:
// * to be included in the structural schema of the CRD.
// * to edit the CreationTimestamp to be nullable and a pointer to metav1.Time instead of a struct which allows
// better serialization.
// * remove ManagedFields which contain unsupported "Any" type.
type ObjectMeta struct {
	// Name must be unique within a namespace. Is required when creating resources, although
	// some resources may allow a client to request the generation of an appropriate name
	// automatically. Name is primarily intended for creation idempotence and configuration
	// definition.
	// Cannot be updated.
	// More info: http://kubernetes.io/docs/user-guide/identifiers#names
	// +optional
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`

	// GenerateName is an optional prefix, used by the server, to generate a unique
	// name ONLY IF the Name field has not been provided.
	// If this field is used, the name returned to the client will be different
	// than the name passed. This value will also be combined with a unique suffix.
	// The provided value has the same validation rules as the Name field,
	// and may be truncated by the length of the suffix required to make the value
	// unique on the server.
	//
	// If this field is specified and the generated name exists, the server will
	// NOT return a 409 - instead, it will either return 201 Created or 500 with Reason
	// ServerTimeout indicating a unique name could not be found in the time allotted, and the client
	// should retry (optionally after the time indicated in the Retry-After header).
	//
	// Applied only if Name is not specified.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#idempotency
	// +optional
	GenerateName string `json:"generateName,omitempty" protobuf:"bytes,2,opt,name=generateName"`

	// Namespace defines the space within each name must be unique. An empty namespace is
	// equivalent to the "default" namespace, but "default" is the canonical representation.
	// Not all objects are required to be scoped to a namespace - the value of this field for
	// those objects will be empty.
	//
	// Must be a DNS_LABEL.
	// Cannot be updated.
	// More info: http://kubernetes.io/docs/user-guide/namespaces
	// +optional
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,3,opt,name=namespace"`

	// SelfLink is a URL representing this object.
	// Populated by the system.
	// Read-only.
	//
	// DEPRECATED
	// Kubernetes will stop propagating this field in 1.20 release and the field is planned
	// to be removed in 1.21 release.
	// +optional
	SelfLink string `json:"selfLink,omitempty" protobuf:"bytes,4,opt,name=selfLink"`

	// UID is the unique in time and space value for this object. It is typically generated by
	// the server on successful creation of a resource and is not allowed to change on PUT
	// operations.
	//
	// Populated by the system.
	// Read-only.
	// More info: http://kubernetes.io/docs/user-guide/identifiers#uids
	// +optional
	UID types.UID `json:"uid,omitempty" protobuf:"bytes,5,opt,name=uid,casttype=k8s.io/kubernetes/pkg/types.UID"`

	// An opaque value that represents the internal version of this object that can
	// be used by clients to determine when objects have changed. May be used for optimistic
	// concurrency, change detection, and the watch operation on a resource or set of resources.
	// Clients must treat these values as opaque and passed unmodified back to the server.
	// They may only be valid for a particular resource or set of resources.
	//
	// Populated by the system.
	// Read-only.
	// Value must be treated as opaque by clients and .
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#concurrency-control-and-consistency
	// +optional
	ResourceVersion string `json:"resourceVersion,omitempty" protobuf:"bytes,6,opt,name=resourceVersion"`

	// A sequence number representing a specific generation of the desired state.
	// Populated by the system. Read-only.
	// +optional
	Generation int64 `json:"generation,omitempty" protobuf:"varint,7,opt,name=generation"`

	// CreationTimestamp is a timestamp representing the server time when this object was
	// created. It is not guaranteed to be set in happens-before order across separate operations.
	// Clients may not set this value. It is represented in RFC3339 form and is in UTC.
	//
	// Populated by the system.
	// Read-only.
	// Null for lists.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	// +nullable
	CreationTimestamp *metav1.Time `json:"creationTimestamp,omitempty" protobuf:"bytes,8,opt,name=creationTimestamp"`

	// DeletionTimestamp is RFC 3339 date and time at which this resource will be deleted. This
	// field is set by the server when a graceful deletion is requested by the user, and is not
	// directly settable by a client. The resource is expected to be deleted (no longer visible
	// from resource lists, and not reachable by name) after the time in this field, once the
	// finalizers list is empty. As long as the finalizers list contains items, deletion is blocked.
	// Once the deletionTimestamp is set, this value may not be unset or be set further into the
	// future, although it may be shortened or the resource may be deleted prior to this time.
	// For example, a user may request that a pod is deleted in 30 seconds. The Kubelet will react
	// by sending a graceful termination signal to the containers in the pod. After that 30 seconds,
	// the Kubelet will send a hard termination signal (SIGKILL) to the container and after cleanup,
	// remove the pod from the API. In the presence of network partitions, this object may still
	// exist after this timestamp, until an administrator or automated process can determine the
	// resource is fully terminated.
	// If not set, graceful deletion of the object has not been requested.
	//
	// Populated by the system when a graceful deletion is requested.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	DeletionTimestamp *metav1.Time `json:"deletionTimestamp,omitempty" protobuf:"bytes,9,opt,name=deletionTimestamp"`

	// Number of seconds allowed for this object to gracefully terminate before
	// it will be removed from the system. Only set when deletionTimestamp is also set.
	// May only be shortened.
	// Read-only.
	// +optional
	DeletionGracePeriodSeconds *int64 `json:"deletionGracePeriodSeconds,omitempty" protobuf:"varint,10,opt,name=deletionGracePeriodSeconds"`

	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: http://kubernetes.io/docs/user-guide/labels
	// +optional
	Labels map[string]string `json:"labels,omitempty" protobuf:"bytes,11,rep,name=labels"`

	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: http://kubernetes.io/docs/user-guide/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty" protobuf:"bytes,12,rep,name=annotations"`

	// List of objects depended by this object. If ALL objects in the list have
	// been deleted, this object will be garbage collected. If this object is managed by a controller,
	// then an entry in this list will point to this controller, with the controller field set to true.
	// There cannot be more than one managing controller.
	// +optional
	// +patchMergeKey=uid
	// +patchStrategy=merge
	OwnerReferences []metav1.OwnerReference `json:"ownerReferences,omitempty" patchStrategy:"merge" patchMergeKey:"uid" protobuf:"bytes,13,rep,name=ownerReferences"`

	// Must be empty before the object is deleted from the registry. Each entry
	// is an identifier for the responsible component that will remove the entry
	// from the list. If the deletionTimestamp of the object is non-nil, entries
	// in this list can only be removed.
	// Finalizers may be processed and removed in any order.  Order is NOT enforced
	// because it introduces significant risk of stuck finalizers.
	// finalizers is a shared field, any actor with permission can reorder it.
	// If the finalizer list is processed in order, then this can lead to a situation
	// in which the component responsible for the first finalizer in the list is
	// waiting for a signal (field value, external system, or other) produced by a
	// component responsible for a finalizer later in the list, resulting in a deadlock.
	// Without enforced ordering finalizers are free to order amongst themselves and
	// are not vulnerable to ordering changes in the list.
	// +optional
	// +patchStrategy=merge
	Finalizers []string `json:"finalizers,omitempty" patchStrategy:"merge" protobuf:"bytes,14,rep,name=finalizers"`

	// The name of the cluster which the object belongs to.
	// This is used to distinguish resources with same name and namespace in different clusters.
	// This field is not set anywhere right now and apiserver is going to ignore it if set in create or update request.
	// +optional
	ClusterName string `json:"clusterName,omitempty" protobuf:"bytes,15,opt,name=clusterName"`
}

type SeldonPodSpec struct {
	Metadata ObjectMeta              `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec     v1.PodSpec              `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	HpaSpec  *SeldonHpaSpec          `json:"hpaSpec,omitempty" protobuf:"bytes,3,opt,name=hpaSpec"`
	Replicas *int32                  `json:"replicas,omitempty" protobuf:"bytes,4,opt,name=replicas"`
	KedaSpec *SeldonScaledObjectSpec `json:"kedaSpec,omitempty" protobuf:"bytes,5,opt,name=kedaSpec"`
	PdbSpec  *SeldonPdbSpec          `json:"pdbSpec,omitempty" protobuf:"bytes,6,opt,name=pdbSpec"`
}

// SeldonScaledObjectSpec is the spec for a KEDA ScaledObject resource
type SeldonScaledObjectSpec struct {
	// +optional
	PollingInterval *int32 `json:"pollingInterval,omitempty" protobuf:"int,1,opt,name=pollingInterval"`
	// +optional
	CooldownPeriod *int32 `json:"cooldownPeriod,omitempty" protobuf:"int,2,opt,name=cooldownPeriod"`
	// +optional
	MinReplicaCount *int32 `json:"minReplicaCount,omitempty" protobuf:"int,3,opt,name=minReplicaCount"`
	// +optional
	MaxReplicaCount *int32 `json:"maxReplicaCount,omitempty" protobuf:"int,4,opt,name=maxReplicaCount"`
	// +optional
	Advanced *kedav1alpha1.AdvancedConfig `json:"advanced,omitempty" protobuf:"bytes,5,opt,name=advanced"`
	Triggers []kedav1alpha1.ScaleTriggers `json:"triggers" protobuf:"bytes,6,opt,name=triggers"`
}

type SeldonHpaSpec struct {
	MinReplicas *int32                          `json:"minReplicas,omitempty" protobuf:"int,1,opt,name=minReplicas"`
	MaxReplicas int32                           `json:"maxReplicas" protobuf:"int,2,opt,name=maxReplicas"`
	Metrics     []autoscalingv2beta2.MetricSpec `json:"metrics,omitempty" protobuf:"bytes,3,opt,name=metrics"`
}

type SeldonPdbSpec struct {
	// An eviction is allowed if at least "minAvailable" pods in the deployment
	// corresponding to a componentSpec will still be available after the eviction, i.e. even in the
	// absence of the evicted pod.  So for example you can prevent all voluntary
	// evictions by specifying "100%".
	// +optional
	MinAvailable *intstr.IntOrString `json:"minAvailable,omitempty" protobuf:"bytes,1,opt,name=minAvailable"`

	// An eviction is allowed if at most "maxUnavailable" pods in the deployment
	// corresponding to a componentSpec are unavailable after the eviction, i.e. even in absence of
	// the evicted pod. For example, one can prevent all voluntary evictions
	// by specifying 0.
	// MaxUnavailable and MinAvailable are mutually exclusive.
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty" protobuf:"bytes,2,opt,name=maxUnavailable"`
}

type PredictiveUnitType string

const (
	UNKNOWN_TYPE       PredictiveUnitType = "UNKNOWN_TYPE"
	ROUTER             PredictiveUnitType = "ROUTER"
	COMBINER           PredictiveUnitType = "COMBINER"
	MODEL              PredictiveUnitType = "MODEL"
	TRANSFORMER        PredictiveUnitType = "TRANSFORMER"
	OUTPUT_TRANSFORMER PredictiveUnitType = "OUTPUT_TRANSFORMER"
)

type PredictiveUnitImplementation string

const (
	UNKNOWN_IMPLEMENTATION PredictiveUnitImplementation = "UNKNOWN_IMPLEMENTATION"
	SIMPLE_MODEL           PredictiveUnitImplementation = "SIMPLE_MODEL"
	SIMPLE_ROUTER          PredictiveUnitImplementation = "SIMPLE_ROUTER"
	RANDOM_ABTEST          PredictiveUnitImplementation = "RANDOM_ABTEST"
	AVERAGE_COMBINER       PredictiveUnitImplementation = "AVERAGE_COMBINER"
)

type PredictiveUnitMethod string

const (
	TRANSFORM_INPUT  PredictiveUnitMethod = "TRANSFORM_INPUT"
	TRANSFORM_OUTPUT PredictiveUnitMethod = "TRANSFORM_OUTPUT"
	ROUTE            PredictiveUnitMethod = "ROUTE"
	AGGREGATE        PredictiveUnitMethod = "AGGREGATE"
	SEND_FEEDBACK    PredictiveUnitMethod = "SEND_FEEDBACK"
)

type EndpointType string

const (
	REST EndpointType = "REST"
	GRPC EndpointType = "GRPC"
)

type Endpoint struct {
	ServiceHost string       `json:"service_host,omitempty" protobuf:"string,1,opt,name=service_host"`
	ServicePort int32        `json:"service_port,omitempty" protobuf:"int32,2,opt,name=service_port"`
	Type        EndpointType `json:"type,omitempty" protobuf:"int,3,opt,name=type"`
	HttpPort    int32        `json:"httpPort,omitempty" protobuf:"int32,4,opt,name=httpPort"`
	GrpcPort    int32        `json:"grpcPort,omitempty" protobuf:"int32,5,opt,name=grpcPort"`
}

type ParmeterType string

const (
	INT    ParmeterType = "INT"
	FLOAT  ParmeterType = "FLOAT"
	DOUBLE ParmeterType = "DOUBLE"
	STRING ParmeterType = "STRING"
	BOOL   ParmeterType = "BOOL"
)

type Parameter struct {
	Name  string       `json:"name" protobuf:"string,1,opt,name=name"`
	Value string       `json:"value" protobuf:"string,2,opt,name=value"`
	Type  ParmeterType `json:"type" protobuf:"int,3,opt,name=type"`
}

type PredictiveUnit struct {
	Name                    string                        `json:"name" protobuf:"string,1,opt,name=name"`
	Children                []PredictiveUnit              `json:"children,omitempty" protobuf:"bytes,2,opt,name=children"`
	Type                    *PredictiveUnitType           `json:"type,omitempty" protobuf:"int,3,opt,name=type"`
	Implementation          *PredictiveUnitImplementation `json:"implementation,omitempty" protobuf:"int,4,opt,name=implementation"`
	Methods                 *[]PredictiveUnitMethod       `json:"methods,omitempty" protobuf:"int,5,opt,name=methods"`
	Endpoint                *Endpoint                     `json:"endpoint,omitempty" protobuf:"bytes,6,opt,name=endpoint"`
	Parameters              []Parameter                   `json:"parameters,omitempty" protobuf:"bytes,7,opt,name=parameters"`
	ModelURI                string                        `json:"modelUri,omitempty" protobuf:"bytes,8,opt,name=modelUri"`
	ServiceAccountName      string                        `json:"serviceAccountName,omitempty" protobuf:"bytes,9,opt,name=serviceAccountName"`
	EnvSecretRefName        string                        `json:"envSecretRefName,omitempty" protobuf:"bytes,10,opt,name=envSecretRefName"`
	StorageInitializerImage string                        `json:"storageInitializerImage,omitempty" protobuf:"bytes,11,opt,name=storageInitializerImage"`
	Logger                  *Logger                       `json:"logger,omitempty" protobuf:"bytes,12,opt,name=logger"`
}

type LoggerMode string

const (
	LogAll      LoggerMode = "all"
	LogRequest  LoggerMode = "request"
	LogResponse LoggerMode = "response"
)

// Logger provides optional payload logging for all endpoints
// +experimental
type Logger struct {
	// URL to send request logging CloudEvents
	// +optional
	Url *string `json:"url,omitempty"`
	// What payloads to log
	Mode LoggerMode `json:"mode,omitempty"`
}

// +genclient
// +genclient:noStatus
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:storageversion
// SeldonDeployment is the Schema for the seldondeployments API
// +k8s:openapi-gen=true
// +kubebuilder:resource:shortName=sdep
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas
type SeldonDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SeldonDeploymentSpec   `json:"spec,omitempty"`
	Status SeldonDeploymentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SeldonDeploymentList contains a list of SeldonDeployment
type SeldonDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SeldonDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SeldonDeployment{}, &SeldonDeploymentList{})
}
