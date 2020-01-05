package cmd

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/watch"
	"k8s.io/kubernetes/pkg/client/conditions"
	"time"
)

type SSHOptions struct {
	configFlags *genericclioptions.ConfigFlags
	rawConfig   api.Config
	client      *kubernetes.Clientset

	args       []string
	Privileged *bool
	genericclioptions.IOStreams
}

var (
	sshExample = `
	# ssh to one node
	%[1]s ssh node01

	%[1]s ssh node02
	`

	errNoContext = fmt.Errorf("no context is currently set, use %q to select a new one", "kubectl config use-context <context>")
)

func (o *SSHOptions) Validate() error {
	if len(o.rawConfig.CurrentContext) == 0 {
		return errNoContext
	}
	if len(o.args) < 1 {
		return fmt.Errorf("must specified one node")
	}

	err := o.checkNode(o.args[0])
	if err != nil {
		return err
	}

	return nil
}

func (o *SSHOptions) Complete(cmd *cobra.Command, args []string) error {
	o.args = args

	var err error
	o.rawConfig, err = o.configFlags.ToRawKubeConfigLoader().RawConfig()
	clientConfig, err := o.configFlags.ToRawKubeConfigLoader().ClientConfig()
	o.client, err = kubernetes.NewForConfig(clientConfig)

	if err != nil {
		return err
	}
	return nil
}

func (o *SSHOptions) Run() error {
	pod := o.createAgentPod()
	pod, err := o.client.CoreV1().Pods("default").Create(context.TODO(), pod, v1.CreateOptions{})
	if err != nil {
		return err
	}
	watcher, err := o.client.CoreV1().Pods(pod.Namespace).Watch(context.TODO(),v1.SingleObject(pod.ObjectMeta))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	fmt.Fprintf(o.Out, "Waiting for pod %s to run...\n", pod.Name)
	event, err := watch.UntilWithoutRetry(ctx, watcher, conditions.PodRunning)
	if err != nil {
		fmt.Fprintf(o.ErrOut, "Error occurred while waiting for pod to run:  %v\n", err)
		return err
	}
	pod = event.Object.(*corev1.Pod)
	return nil
}

func (o *SSHOptions) checkNode(node string) error {
	nodes, err := o.client.CoreV1().Nodes().List(context.TODO(), v1.ListOptions{})
	if err != nil {
		return err
	}
	for _, value := range nodes.Items {
		if value.Name == node {
			return nil
		}
	}

	return fmt.Errorf("cannot find specified node %s, use kubectl get node to confirm", o.args[0])
}

func NewSshOptions(streams genericclioptions.IOStreams) *SSHOptions {
	b := true
	return &SSHOptions{
		configFlags: genericclioptions.NewConfigFlags(true),

		IOStreams:  streams,
		Privileged: &b,
	}
}

func NewCmdSsh(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewSshOptions(streams)

	cmd := &cobra.Command{
		Use:          "ssh [nodeName] [flags]",
		Short:        "ssh to specified k8s host node",
		Example:      fmt.Sprintf(sshExample, "kubectl"),
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			if err := o.Complete(c, args); err != nil {
				return err
			}
			if err := o.Validate(); err != nil {
				return err
			}
			if err := o.Run(); err != nil {
				return err
			}
			return nil
		},
	}
	return cmd
}

func (o *SSHOptions) createAgentPod() *corev1.Pod {
	agentPod := &corev1.Pod{
		TypeMeta: v1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "ssh-agent",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeName: o.args[0],
			HostPID:  true,
			Containers: []corev1.Container{
				{
					Name:            "ssh-agent",
					Image:           "busybox",
					ImagePullPolicy: corev1.PullAlways,
					SecurityContext: &corev1.SecurityContext{Privileged: o.Privileged},
					Command:         []string{"nsenter", "--target", "1", "--mount", "--uts", "--ipc", "--net", "--pid", "--", "bash", "-l"},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
	return agentPod
}
