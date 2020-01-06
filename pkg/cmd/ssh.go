package cmd

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/zxh326/kubectl-ssh/pkg/util"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd/api"
	"os/exec"
)

type SSHOptions struct {
	configFlags *genericclioptions.ConfigFlags
	rawConfig   api.Config
	client      *kubernetes.Clientset

	args       []string
	nodeName   string
	agentName  string
	agentImage string
	label      string

	Privileged *bool
	genericclioptions.IOStreams
}

var (
	sshExample = `
	# ssh to one node
	%[1]s ssh node01

	%[1]s ssh node02
	`

	overrides = `
	{
	  "spec": {
		"nodeName": "%s",
		"hostPID": true,
		"containers": [
		  {
			"securityContext": {
			  "privileged": true
			},
			"image": "%s",
			"name": "nsenter",
			"stdin": true,
			"stdinOnce": true,
			"tty": true,
			"command": [ "nsenter", "--target", "1", "--mount", "--uts", "--ipc", "--net", "--pid", "--", "bash", "-l" ]
		  }
		]
	  }
	}
	`

	errNoContext = fmt.Errorf("no context is currently set, use %q to select a new one", "kubectl config use-context <context>")
)

func NewSshOptions(streams genericclioptions.IOStreams) *SSHOptions {
	b := true
	return &SSHOptions{
		configFlags: genericclioptions.NewConfigFlags(true),
		agentImage:  "busybox",
		IOStreams:   streams,
		Privileged:  &b,
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
	cmd.Flags().StringVarP(&o.label, "label", "l", o.label,
		"ssh node with label name")
	o.configFlags.AddFlags(cmd.Flags())

	return cmd
}

func (o *SSHOptions) Validate() error {
	if len(o.rawConfig.CurrentContext) == 0 {
		return errNoContext
	}
	if len(o.args) < 1 && o.label == "" {
		return fmt.Errorf("must specified one node")
	}

	err := o.checkNode()
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
	o.agentName = util.RandomString("ssh-agent-", 6)
	cmd := exec.Command(
		"kubectl",
		"run", o.agentName,
		"--rm",
		"-it",
		"--image", o.agentImage,
		"--generator", "run-pod/v1",
		"--overrides", fmt.Sprintf(overrides, o.nodeName, o.agentImage),
	)
	cmd.Stdin = o.In
	cmd.Stdout = o.Out
	cmd.Stderr = o.ErrOut
	return cmd.Run()
}

func (o *SSHOptions) checkNode() error {
	listOptions := v1.ListOptions{}
	if o.label != "" {
		listOptions.LabelSelector = o.label
	}
	nodes, err := o.client.CoreV1().Nodes().List(context.TODO(), listOptions)
	if err != nil {
		return err
	}
	for _, value := range nodes.Items {
		if o.label != "" {
			o.nodeName = value.Name
			return nil
		}
		if len(o.args) > 0 {
			if value.Name == o.args[0] {
				o.nodeName = value.Name
				return nil
			}
		}
	}
	return fmt.Errorf("cannot find specified node %s, use kubectl get node to confirm", o.args[0])
}

// Deprecated
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
