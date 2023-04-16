## Upgrade Version

Delete all files

```
kubebuilder init --domain nakamasato.com --repo github.com/nakamasato/secret-mirror-operator
```

```
git checkout README.md renovate.json
```

```
kubebuilder create api --group secret --version v1alpha1 --kind SecretMirror --resource --controller
make manifests
make generate
```

update `api/v1alpha1/secretmirror_types.go`:

```go
type SecretMirrorSpec struct {
	// FromNamespace is a namespace from which the target Secret is mirrored.
	// +kubebuilder:validation:Required
	FromNamespace string `json:"fromNamespace"`
}
```

`config/samples/secret_v1alpha1_secretmirror.yaml`:
```yaml
  name: secret
  namespace: dst
spec:
  fromNamespace: src
```

update controller.go and controller_test.go
