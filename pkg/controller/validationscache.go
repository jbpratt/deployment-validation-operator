package controller

import (
	"github.com/app-sre/deployment-validation-operator/pkg/validations"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type validationKey struct {
	group, version, kind, namespace, name string
}

type resourceVersion string

func newResourceversionVal(str string) resourceVersion {
	return resourceVersion(str)
}

// newValidationKey returns a unique identifier for the given
// object suitable for hashing.
func newValidationKey(obj client.Object) validationKey {
	gvk := obj.GetObjectKind().GroupVersionKind()
	return validationKey{
		group:     gvk.Group,
		version:   gvk.Version,
		kind:      gvk.Kind,
		namespace: obj.GetNamespace(),
		name:      obj.GetName(),
	}
}

type validationResource struct {
	version resourceVersion
	uid     string
	outcome validations.ValidationOutcome
}

// newValidationResource returns a 'validationResource' populated
// with the given 'resourceVersion', 'uid', and 'ValidationOutcome'.
func newValidationResource(
	rscVer resourceVersion,
	uid string,
	outcome validations.ValidationOutcome,
) validationResource {
	return validationResource{
		uid:     uid,
		version: rscVer,
		outcome: outcome,
	}
}

type validationCache map[validationKey]validationResource

// newValidationCache returns a new empty instance of validationCache struct
func newValidationCache() *validationCache {
	return &validationCache{}
}

// has returns 'true' if the given key exist in the instance; 'false' otherwise.
func (vc *validationCache) has(key validationKey) bool {
	_, exists := (*vc)[key]
	return exists
}

// store caches a 'ValidationOutcome' for the given 'Object'.
// constraint: cached outcomes will be updated in-place for a given object and
// consecutive updates will not preserve previous state.
func (vc *validationCache) store(obj client.Object, outcome validations.ValidationOutcome) {
	key := newValidationKey(obj)
	(*vc)[key] = newValidationResource(
		newResourceversionVal(obj.GetResourceVersion()),
		string(obj.GetUID()),
		outcome,
	)
}

// drain frees the cache of any used resources
// resulting in all cached 'ValidationOutcome's being lost.
func (vc *validationCache) drain() {
	*vc = validationCache{}
}

// remove uncaches the 'ValidationOutcome' for the
// given object if it exists and performs a noop
// if it does not.
func (vc *validationCache) remove(obj client.Object) {
	key := newValidationKey(obj)
	vc.removeKey(key)
}

// removeKey deletes a key, and its value, from the instance
func (vc *validationCache) removeKey(key validationKey) {
	delete(*vc, key)
}

// retrieve returns a tuple of 'validationResource' (if present)
// and 'ok' which returns 'true' if a 'validationResource' exists
// for the given 'Object' and 'false' otherwise.
func (vc *validationCache) retrieve(obj client.Object) (validationResource, bool) {
	key := newValidationKey(obj)
	val, exists := (*vc)[key]
	return val, exists
}

// objectAlreadyValidated returns 'true' if the given 'Object'
// has a cached 'ValidationOutcome' with the same 'ResourceVersion'
// (Kubernetes representation of iteration count for a persisted resource).
// If the 'ResourceVersion' of an existing 'Object' is stale the cached
// 'ValidationOutcome' is removed and 'false' is returned. In all other
// cases 'false' is returned.
func (vc *validationCache) objectAlreadyValidated(obj client.Object) bool {
	validationOutcome, ok := vc.retrieve(obj)
	storedResourceVersion := validationOutcome.version
	if !ok {
		return false
	}
	currentResourceVersion := obj.GetResourceVersion()
	if string(storedResourceVersion) != currentResourceVersion {
		vc.remove(obj)
		return false
	}
	return true
}
