package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/vault/api"
	"github.com/kelseyhightower/envconfig"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Config is the struct for environment configuration
type Config struct {
	VaultAddr       string `envconfig:"VAULT_ADDR"`
	K8sNamespace    string `envconfig:"K8S_NAMESPACE"`
	VaultSecretPath string `envconfig:"VAULT_SECRET_PATH"`
	RoleID          string `envconfig:"VAULT_ROLE_ID"`
	SecretID        string `envconfig:"VAULT_SECRET_ID"`
	KvVersion       int    `envconfig:"VAULT_KV_VERSION"`
	Local           bool   `envconfig:"LOCAL"`
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

// used for local testing only
func getK8sClientSetOutsideClusterConfig() *kubernetes.Clientset {
	// use the current context in kubeconfig to create the clientset
	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	return clientset
}

func getK8sClientSetInClusterConfig() *kubernetes.Clientset {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	return clientset
}

func getK8sClientSet(isLocal bool) *kubernetes.Clientset {
	if isLocal {
		return getK8sClientSetOutsideClusterConfig()
	}
	return getK8sClientSetInClusterConfig()
}

func buildK8sSecret(name string, data map[string][]byte) corev1.Secret {
	return corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Data: data,
		Type: "Opaque",
	}
}

func upcertSecret(c *kubernetes.Clientset, name, namespace string, secret *corev1.Secret) error {
	_, err := c.CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		log.Println(err)
		if strings.Index(err.Error(), "not found") >= 0 {
			log.Printf("Secret %s doesn't exist, creating ...\n", name)
			_, err = c.CoreV1().Secrets(namespace).Create(secret)
			if err != nil {
				return err
			}
			log.Printf("Secret %s created!\n", name)
		}
	} else {
		log.Printf("Secret %s exist, updating ...\n", name)
		_, err = c.CoreV1().Secrets(namespace).Update(secret)
		if err != nil {
			return err
		}
		log.Printf("Secret %s updated!\n", name)
	}
	return nil
}

func main() {
	// load config
	var conf Config
	err := envconfig.Process("", &conf)
	if err != nil {
		log.Fatal(err.Error())
	}

	// kv v2 only
	listSecretPath := fmt.Sprintf("%s/metadata/", conf.VaultSecretPath)
	readSecretPath := fmt.Sprintf("%s/data", conf.VaultSecretPath)

	// vault client
	log.Println("Creating vault client ...")
	vaultClient, err := api.NewClient(&api.Config{
		Address: conf.VaultAddr,
	})
	if err != nil {
		log.Fatal(err.Error())
		return
	}
	// AppRole auth
	c := vaultClient.Logical()
	data := map[string]interface{}{
		"role_id":   conf.RoleID,
		"secret_id": conf.SecretID,
	}
	log.Println("App role auth ...")
	resp, err := c.Write("auth/approle/login", data)
	if err != nil {
		log.Fatal(err.Error())
		return
	}
	if resp.Auth == nil {
		log.Fatal("no auth info returned")
		return
	}
	// set token after AppRole auth
	log.Println("App role auth succeeded, set token ...")
	vaultClient.SetToken(resp.Auth.ClientToken)

	// list vault secrets
	log.Println("Listing all secrets from vault ...")
	vaultSecrets, err := c.List(listSecretPath)
	if err != nil {
		log.Fatal(err.Error())
		return
	}

	// k8s client
	log.Println("Creating k8s client ...")
	k8sClientset := getK8sClientSet(conf.Local)

	// iterate all vault secrets, generate k8s secret, and upcert
	log.Println("Starting to create/update k8s secrets ...")
	switch x := vaultSecrets.Data["keys"].(type) {
	case []interface{}:
		for _, k := range x {
			secretName := fmt.Sprintf("%v", k)
			log.Printf("Processing secret %s ...\n", secretName)
			secret, err := c.Read(fmt.Sprintf("%s/%s", readSecretPath, secretName))
			if err != nil {
				log.Fatal(err.Error())
				return
			}

			data := make(map[string][]byte)
			for k, v := range secret.Data["data"].(map[string]interface{}) {
				data[k] = []byte(fmt.Sprintf("%v", v))
			}

			// upcert
			k8sSecret := buildK8sSecret(secretName, data)
			err = upcertSecret(k8sClientset, secretName, conf.K8sNamespace, &k8sSecret)
			if err != nil {
				log.Fatal(err.Error())
				panic(err.Error())
			}
		}
	}
	log.Println("Create/update k8s secrets done!...")
}
