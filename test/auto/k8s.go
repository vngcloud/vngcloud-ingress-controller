package test

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	KUBE_CONFIG_PATH = filepath.Join(os.Getenv("HOME"), ".kube", "config")
)

func createK8sClient() (*kubernetes.Clientset, error) {
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", KUBE_CONFIG_PATH)
	if err != nil {
		logrus.Errorf("Error getting kubernetes config: %v\n", err)
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(kubeConfig)

	if err != nil {
		logrus.Errorf("error getting kubernetes config: %v\n", err)
		return nil, err
	}
	return clientset, nil
}

func ListPods(namespace string, client kubernetes.Interface) (*v1.PodList, error) {
	fmt.Println("Get Kubernetes Pods")
	pods, err := client.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		err = fmt.Errorf("error getting pods: %v\n", err)
		return nil, err
	}
	return pods, nil
}

func ListNamespaces(client kubernetes.Interface) (*v1.NamespaceList, error) {
	fmt.Println("Get Kubernetes Namespaces")
	namespaces, err := client.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		err = fmt.Errorf("error getting namespaces: %v\n", err)
		return nil, err
	}
	return namespaces, nil
}

func ListNodes(client kubernetes.Interface) (*v1.NodeList, error) {
	fmt.Println("Get Kubernetes Nodes")
	nodes, err := client.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		err = fmt.Errorf("error getting nodes: %v\n", err)
		return nil, err
	}
	return nodes, nil
}

func ApplyYAML(yamlData string) {
	logrus.Infoln("######################## ApplyYAML ########################")
	fileName := "_test.yaml"
	// write to file
	if err := os.WriteFile(fileName, []byte(yamlData), 0666); err != nil {
		log.Fatal(err)
	}
	DoCommand(fmt.Sprintf("kubectl apply -f %s", fileName))
	// delete file
	defer os.Remove(fileName)
}

func DeleteYAML(yamlData string) {
	logrus.Infoln("######################## DeleteYAML ########################")
	fileName := "_test.yaml"
	// write to file
	if err := os.WriteFile(fileName, []byte(yamlData), 0666); err != nil {
		logrus.Errorln("error writing file yaml:", err)
	}
	DoCommand(fmt.Sprintf("kubectl delete -f %s", fileName))
	// delete file
	err := os.Remove(fileName)
	if err != nil {
		logrus.Errorln("error delete file yaml:", err)
	}
}
