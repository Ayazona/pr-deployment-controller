package internal

import "fmt"

// GenerateBuildURL creates the build URL that exposes the test-environment (without protocol prefix)
func GenerateBuildURL(
	owner, repository string, pullRequestNumber int64, clusterDomain string,
) string {
	// dont include the owner in the url to reduce the url length
	buildName := fmt.Sprintf(
		"%s-%d",
		repository,
		pullRequestNumber,
	)

	return fmt.Sprintf("%s.%s", buildName, clusterDomain)
}

// GenerateLogsURL creates the url to access environment logs (without protocol prefix)
func GenerateLogsURL(
	buildPrefix, owner, repository string, pullRequestNumber int64, kibanaURL string,
) string {
	namespace := fmt.Sprintf(
		"%s%s-%s-%d",
		buildPrefix,
		owner,
		repository,
		pullRequestNumber,
	)

	return fmt.Sprintf(
		"%s/app/kibana#/discover?_g=()&_a=(columns:!(_source),filters:!(('$state':(store:appState),meta:"+
			"(alias:!n,disabled:!f,index:e571f600-440f-11e9-8996-e733dd7babc6,key:kubernetes.namespace_name,negate:!f"+
			",params:(query:%s,type:phrase),type:phrase,value:%s),query:(match:(kubernetes.namespace_name:(query:%s,"+
			"type:phrase))))),index:e571f600-440f-11e9-8996-e733dd7babc6,interval:auto,query:(language:lucene,query:'')"+
			",sort:!('@timestamp',desc))",
		kibanaURL,
		namespace,
		namespace,
		namespace,
	)
}
