package fastly

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var computeAttributes = ServiceMetadata{
	ServiceTypeCompute,
}

// Ordering is important - stored is processing order
// Some objects may need to be updated first, as they can be referenced by other
// configuration objects (Backends, Request Headers, etc).
var computeService = &BaseServiceDefinition{
	Type: computeAttributes.serviceType,
	Attributes: []ServiceAttributeDefinition{
		NewServiceSettings(),
		NewServiceDomain(computeAttributes),
		NewServiceHealthCheck(computeAttributes),
		NewServiceBackend(computeAttributes),
		NewServiceDirector(computeAttributes),
		NewServiceS3Logging(computeAttributes),
		NewServicePaperTrail(computeAttributes),
		NewServiceSumologic(computeAttributes),
		NewServiceGCSLogging(computeAttributes),
		NewServiceBigQueryLogging(computeAttributes),
		NewServiceSyslog(computeAttributes),
		NewServiceLogentries(computeAttributes),
		NewServiceSplunk(computeAttributes),
		NewServiceBlobStorageLogging(computeAttributes),
		NewServiceHTTPSLogging(computeAttributes),
		NewServiceLoggingElasticSearch(computeAttributes),
		NewServiceLoggingFTP(computeAttributes),
		NewServiceLoggingSFTP(computeAttributes),
		NewServiceLoggingDatadog(computeAttributes),
		NewServiceLoggingLoggly(computeAttributes),
		NewServiceLoggingGooglePubSub(computeAttributes),
		NewServiceLoggingScalyr(computeAttributes),
		NewServiceLoggingNewRelic(computeAttributes),
		NewServiceLoggingKafka(computeAttributes),
		NewServiceLoggingHeroku(computeAttributes),
		NewServiceLoggingHoneycomb(computeAttributes),
		NewServiceLoggingLogshuttle(computeAttributes),
		NewServiceLoggingOpenstack(computeAttributes),
		NewServiceLoggingDigitalOcean(computeAttributes),
		NewServiceLoggingCloudfiles(computeAttributes),
		NewServiceLoggingKinesis(computeAttributes),
		NewServiceDictionary(computeAttributes),
		NewServicePackage(computeAttributes),
	},
}

func resourceServiceComputeV1() *schema.Resource {
	return resourceService(computeService)
}
