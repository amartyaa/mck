module github.com/amartyaa/mck

go 1.22

require (
	github.com/spf13/cobra v1.8.1
	github.com/spf13/viper v1.19.0
	github.com/fatih/color v1.17.0
	gopkg.in/yaml.v3 v3.0.1

	// Cloud Provider SDKs
	github.com/aws/aws-sdk-go-v2 v1.30.3
	github.com/aws/aws-sdk-go-v2/config v1.27.27
	github.com/aws/aws-sdk-go-v2/service/eks v1.46.2
	github.com/aws/aws-sdk-go-v2/service/costexplorer v1.40.2

	cloud.google.com/go/container v1.37.2
	cloud.google.com/go/billing v1.18.6

	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.7.0
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4 v4.9.0

	github.com/oracle/oci-go-sdk/v65 v65.69.2

	google.golang.org/api v0.190.0
)
