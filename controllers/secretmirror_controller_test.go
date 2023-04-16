package controllers

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	secretv1alpha1 "github.com/nakamasato/secret-mirror-operator/api/v1alpha1"
)

const (
	srcNamespace          = "src"
	dstNamespace          = "dst"
	secretName            = "secret"
	nonExistingSecretName = "non-existing-secret"
	interval              = 250 * time.Millisecond
	timeout               = 30 * time.Second
)

var _ = Describe("SecretMirror controller", func() {
	BeforeEach(func() {
		// Create Secret in src Namespace
		secret := newTestSecret(secretName, srcNamespace)
		Expect(k8sClient.Create(ctx, secret, &client.CreateOptions{})).Should(Succeed())
	})

	AfterEach(func() {
		// Delete src Namespace
		err := k8sClient.DeleteAllOf(ctx, &v1.Secret{}, client.InNamespace(srcNamespace))
		Expect(err).NotTo(HaveOccurred())

		// Delete dst Namespace
		err = k8sClient.DeleteAllOf(ctx, &secretv1alpha1.SecretMirror{}, client.InNamespace(dstNamespace))
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &v1.Secret{}, client.InNamespace(dstNamespace))
		Expect(err).NotTo(HaveOccurred())
	})

	It("should create Secret in dst namespace", func() {
		By("Creating SecretMirror")
		secretMirror := newSecretMirror(secretName, dstNamespace)
		Expect(k8sClient.Create(ctx, secretMirror, &client.CreateOptions{})).Should(Succeed())

		// Secret should be created
		toSecret := v1.Secret{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Namespace: dstNamespace, Name: secretName}, &toSecret)
		}, timeout, interval).Should(Succeed())
		Expect(toSecret.Data).Should(Equal(map[string][]byte{"foo": []byte("bar")}))
		boolTrue := true
		expectedOwnerReference := metav1.OwnerReference{
			Kind:               "SecretMirror",
			APIVersion:         "secret.nakamasato.com/v1alpha1",
			UID:                secretMirror.UID,
			Name:               secretName,
			Controller:         &boolTrue,
			BlockOwnerDeletion: &boolTrue,
		}
		Expect(toSecret.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference))
	})

	It("should keep Secret in dst namespace same as the original one", func() {
		By("Creating SecretMirror")
		secretMirror := newSecretMirror(secretName, dstNamespace)
		Expect(k8sClient.Create(ctx, secretMirror, &client.CreateOptions{})).Should(Succeed())

		// Secret should be created
		toSecret := v1.Secret{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Namespace: dstNamespace, Name: secretName}, &toSecret)
		}, timeout, interval).Should(Succeed())

		By("Updating Secret in dst namespace")
		toSecret.Data = map[string][]byte{"updated": []byte("true")}
		Expect(k8sClient.Update(ctx, &toSecret, &client.UpdateOptions{})).Should(Succeed())

		toSecret = v1.Secret{}
		Eventually(func() map[string][]byte {
			err := k8sClient.Get(ctx, client.ObjectKey{Namespace: dstNamespace, Name: secretName}, &toSecret)
			if err != nil {
				return nil
			}
			return toSecret.Data
		}, timeout, interval).Should(Equal(map[string][]byte{"foo": []byte("bar")}))
	})

	It("should delete toSecret in dst namespace", func() {
		By("Creating SecretMirror")
		secretMirror := newSecretMirror(secretName, dstNamespace)
		Expect(k8sClient.Create(ctx, secretMirror, &client.CreateOptions{})).Should(Succeed())

		// Secret should be created
		toSecret := v1.Secret{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Namespace: dstNamespace, Name: secretName}, &toSecret)
		}, timeout, interval).Should(Succeed())

		By("Deleting Secret in src namespace")
		fromSecret := v1.Secret{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: srcNamespace, Name: secretName}, &fromSecret)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, &fromSecret, &client.DeleteOptions{})).Should(Succeed())

		// toSecret is deleted as fromSecret is deleted
		toSecret = v1.Secret{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Namespace: dstNamespace, Name: secretName}, &toSecret)
		}, timeout, interval).ShouldNot(Succeed())
	})

	It("should not delete Secret in dst namespace", func() {
		By("Secret already exists in dst namespace")
		secret := newTestSecret(secretName, dstNamespace)
		Expect(k8sClient.Create(ctx, secret, &client.CreateOptions{})).Should(Succeed())

		By("Creating SecretMirror")
		secretMirror := newSecretMirror(secretName, dstNamespace)
		Expect(k8sClient.Create(ctx, secretMirror, &client.CreateOptions{})).Should(Succeed())

		By("Deleting Secret in src namespace")
		fromSecret := v1.Secret{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: srcNamespace, Name: secretName}, &fromSecret)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, &fromSecret, &client.DeleteOptions{})).Should(Succeed())

		secret = &v1.Secret{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: dstNamespace, Name: secretName}, secret)).Should(Succeed())
		Expect(secret.ObjectMeta.OwnerReferences).To(BeEmpty()) // Not managed by SecretMirror
	})
})

func newTestSecret(name, namespace string) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{"foo": []byte("bar")}, // echo -n 'bar' | base64 -> YmFy
	}
}

func newNamespace(name string) *v1.Namespace {
	return &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func newSecretMirror(name, namespace string) *secretv1alpha1.SecretMirror {
	return &secretv1alpha1.SecretMirror{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: secretv1alpha1.SecretMirrorSpec{
			FromNamespace: srcNamespace,
		},
		Status: secretv1alpha1.SecretMirrorStatus{},
	}
}
