package v1alpha1

import (
	"fmt"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	apis "code.cloudfoundry.org/quarks-job/pkg/kube/apis"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
)

// This file looks almost the same for all controllers
// Modify the addKnownTypes function, then run `make generate`

const (
	// QuarksJobResourceKind is the kind name of QuarksJob
	QuarksJobResourceKind = "QuarksJob"
	// QuarksJobResourcePlural is the plural name of QuarksJob
	QuarksJobResourcePlural = "quarksjobs"
)

var (
	schemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme is used for schema registrations in the controller package
	// and also in the generated kube code
	AddToScheme = schemeBuilder.AddToScheme

	// QuarksJobResourceShortNames is the short names of QuarksJob
	QuarksJobResourceShortNames = []string{"qjob", "qjobs"}
	// QuarksJobValidation is the validation method for QuarksJob
	QuarksJobValidation = extv1.CustomResourceValidation{
		OpenAPIV3Schema: &extv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]extv1.JSONSchemaProps{
				"spec": {
					Type: "object",
					Properties: map[string]extv1.JSONSchemaProps{
						"output": {
							Type: "object",
							Properties: map[string]extv1.JSONSchemaProps{
								"outputMap": {
									Type:                   "object",
									XPreserveUnknownFields: pointers.Bool(true),
								},
								"outputType": {
									Type: "string",
								},
								"secretLabels": {
									Type:                   "object",
									XPreserveUnknownFields: pointers.Bool(true),
								},
								"writeOnFailure": {
									Type: "boolean",
								},
							},
							Required: []string{
								"outputMap",
							},
						},
						"trigger": {
							Type: "object",
							Properties: map[string]extv1.JSONSchemaProps{
								"strategy": {
									Type: "string",
									Enum: []extv1.JSON{
										{
											Raw: []byte(`"manual"`),
										},
										{
											Raw: []byte(`"once"`),
										},
										{
											Raw: []byte(`"now"`),
										},
										{
											Raw: []byte(`"done"`),
										},
									},
								},
							},
							Required: []string{
								"strategy",
							},
						},
						"template": {
							Type:                   "object",
							XPreserveUnknownFields: pointers.Bool(true),
						},
						"updateOnConfigChange": {
							Type: "boolean",
						},
					},
				},
				"status": {
					Type: "object",
					Properties: map[string]extv1.JSONSchemaProps{
						"lastReconcile": {
							Type:     "string",
							Nullable: true,
						},
						"completed": {
							Type: "boolean",
						},
					},
				},
			},
		},
	}

	// QuarksJobAdditionalPrinterColumns are used by `kubectl get`
	QuarksJobAdditionalPrinterColumns = []extv1.CustomResourceColumnDefinition{
		{
			Name:        "completed",
			Type:        "boolean",
			Description: "",
			Priority:    0,
			JSONPath:    ".status.completed",
		},
		{
			Name:        "strategy",
			Type:        "string",
			Description: "",
			Priority:    10,
			JSONPath:    ".spec.trigger.strategy",
		},
		{
			Name:        "updateOnConfigChange",
			Type:        "boolean",
			Description: "",
			Priority:    20,
			JSONPath:    ".spec.updateOnConfigChange",
		},
	}

	// QuarksJobResourceName is the resource name of QuarksJob
	QuarksJobResourceName = fmt.Sprintf("%s.%s", QuarksJobResourcePlural, apis.GroupName)

	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: apis.GroupName, Version: "v1alpha1"}
)

// Kind takes an unqualified kind and returns back a Group qualified GroupKind
func Kind(kind string) schema.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource takes an unqualified resource and returns a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

// Adds the list of known types to Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&QuarksJob{},
		&QuarksJobList{},
	)

	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
