package fastly

import (
	"context"
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

type S3LoggingServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceS3Logging(sa ServiceMetadata) ServiceAttributeDefinition {
	return ToServiceAttributeDefinition(&S3LoggingServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "s3logging",
			serviceMetadata: sa,
		},
	})
}

func (h *S3LoggingServiceAttributeHandler) Key() string { return h.key }

func (h *S3LoggingServiceAttributeHandler) GetSchema() *schema.Schema {
	var blockAttributes = map[string]*schema.Schema{
		// Required fields
		"name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The unique name of the S3 logging endpoint. It is important to note that changing this attribute will delete and recreate the resource",
		},
		"bucket_name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The name of the bucket in which to store the logs",
		},
		"s3_access_key": {
			Type:        schema.TypeString,
			Optional:    true,
			DefaultFunc: schema.EnvDefaultFunc("FASTLY_S3_ACCESS_KEY", ""),
			Description: "AWS Access Key of an account with the required permissions to post logs. It is **strongly** recommended you create a separate IAM user with permissions to only operate on this Bucket. This key will be not be encrypted. Not required if `iam_role` is provided. You can provide this key via an environment variable, `FASTLY_S3_ACCESS_KEY`",
			Sensitive:   true,
		},
		"s3_secret_key": {
			Type:        schema.TypeString,
			Optional:    true,
			DefaultFunc: schema.EnvDefaultFunc("FASTLY_S3_SECRET_KEY", ""),
			Description: "AWS Secret Key of an account with the required permissions to post logs. It is **strongly** recommended you create a separate IAM user with permissions to only operate on this Bucket. This secret will be not be encrypted. Not required if `iam_role` is provided. You can provide this secret via an environment variable, `FASTLY_S3_SECRET_KEY`",
			Sensitive:   true,
		},
		"s3_iam_role": {
			Type:        schema.TypeString,
			Optional:    true,
			DefaultFunc: schema.EnvDefaultFunc("FASTLY_S3_IAM_ROLE", ""),
			Description: "The Amazon Resource Name (ARN) for the IAM role granting Fastly access to S3. Not required if `access_key` and `secret_key` are provided. You can provide this value via an environment variable, `FASTLY_S3_IAM_ROLE`",
			Sensitive:   false,
		},
		// Optional fields
		"acl": {
			Type:     schema.TypeString,
			Optional: true,
			Description: fmt.Sprintf("The AWS [Canned ACL](https://docs.aws.amazon.com/AmazonS3/latest/userguide/acl-overview.html#canned-acl) to use for objects uploaded to the S3 bucket. Options are: `%s`, `%s`, `%s`, `%s`, `%s`, `%s`, `%s`",
				gofastly.S3AccessControlListPrivate,
				gofastly.S3AccessControlListPublicRead,
				gofastly.S3AccessControlListPublicReadWrite,
				gofastly.S3AccessControlListAWSExecRead,
				gofastly.S3AccessControlListAuthenticatedRead,
				gofastly.S3AccessControlListBucketOwnerRead,
				gofastly.S3AccessControlListBucketOwnerFullControl,
			),
			ValidateDiagFunc: validation.ToDiagFunc(validation.StringInSlice(
				[]string{
					string(gofastly.S3AccessControlListPrivate),
					string(gofastly.S3AccessControlListPublicRead),
					string(gofastly.S3AccessControlListPublicReadWrite),
					string(gofastly.S3AccessControlListAWSExecRead),
					string(gofastly.S3AccessControlListAuthenticatedRead),
					string(gofastly.S3AccessControlListBucketOwnerRead),
					string(gofastly.S3AccessControlListBucketOwnerFullControl),
				},
				false,
			)),
		},
		"path": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "Path to store the files. Must end with a trailing slash. If this field is left empty, the files will be saved in the bucket's root path",
		},
		"domain": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "If you created the S3 bucket outside of `us-east-1`, then specify the corresponding bucket endpoint. Example: `s3-us-west-2.amazonaws.com`",
			Default:     "s3.amazonaws.com",
		},
		"gzip_level": {
			Type:        schema.TypeInt,
			Optional:    true,
			Default:     0,
			Description: "Level of Gzip compression, from `0-9`. `0` is no compression. `1` is fastest and least compressed, `9` is slowest and most compressed. Default `0`",
		},
		"period": {
			Type:        schema.TypeInt,
			Optional:    true,
			Default:     3600,
			Description: "How frequently the logs should be transferred, in seconds. Default `3600`",
		},
		"timestamp_format": {
			Type:        schema.TypeString,
			Optional:    true,
			Default:     "%Y-%m-%dT%H:%M:%S.000",
			Description: "`strftime` specified timestamp formatting (default `%Y-%m-%dT%H:%M:%S.000`)",
		},
		"redundancy": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: fmt.Sprintf("The S3 storage class (redundancy level). Should be one of: `%s`, `%s`, `%s`, or `%s`", gofastly.S3RedundancyStandard, gofastly.S3RedundancyReduced, gofastly.S3RedundancyStandardIA, gofastly.S3RedundancyOneZoneIA),
			ValidateDiagFunc: validation.ToDiagFunc(validation.StringInSlice(
				[]string{
					string(gofastly.S3RedundancyStandard),
					string(gofastly.S3RedundancyReduced),
					string(gofastly.S3RedundancyStandardIA),
					string(gofastly.S3RedundancyOneZoneIA),
				},
				false,
			)),
		},
		"public_key": {
			Type:             schema.TypeString,
			Optional:         true,
			Description:      "A PGP public key that Fastly will use to encrypt your log files before writing them to disk",
			ValidateDiagFunc: validateStringTrimmed,
		},
		"message_type": {
			Type:             schema.TypeString,
			Optional:         true,
			Default:          "classic",
			Description:      "How the message should be formatted; one of: `classic`, `loggly`, `logplex` or `blank`. Default `classic`",
			ValidateDiagFunc: validateLoggingMessageType(),
		},
		"server_side_encryption": {
			Type:             schema.TypeString,
			Optional:         true,
			Description:      "Specify what type of server side encryption should be used. Can be either `AES256` or `aws:kms`",
			ValidateDiagFunc: validateLoggingServerSideEncryption(),
		},
		"server_side_encryption_kms_key_id": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "Optional server-side KMS Key Id. Must be set if server_side_encryption is set to `aws:kms`",
		},
		"compression_codec": {
			Type:             schema.TypeString,
			Optional:         true,
			Description:      `The codec used for compression of your logs. Valid values are zstd, snappy, and gzip. If the specified codec is "gzip", gzip_level will default to 3. To specify a different level, leave compression_codec blank and explicitly set the level using gzip_level. Specifying both compression_codec and gzip_level in the same API request will result in an error.`,
			ValidateDiagFunc: validateLoggingCompressionCodec(),
		},
	}

	if h.GetServiceMetadata().serviceType == ServiceTypeVCL {
		blockAttributes["format"] = &schema.Schema{
			Type:        schema.TypeString,
			Optional:    true,
			Default:     "%h %l %u %t %r %>s",
			Description: "Apache-style string or VCL variables to use for log formatting.",
		}
		blockAttributes["format_version"] = &schema.Schema{
			Type:             schema.TypeInt,
			Optional:         true,
			Default:          1,
			Description:      "The version of the custom logging format used for the configured endpoint. Can be either 1 or 2. (Default: 1).",
			ValidateDiagFunc: validateLoggingFormatVersion(),
		}
		blockAttributes["response_condition"] = &schema.Schema{
			Type:        schema.TypeString,
			Optional:    true,
			Default:     "",
			Description: "Name of blockAttributes condition to apply this logging.",
		}
		blockAttributes["placement"] = &schema.Schema{
			Type:             schema.TypeString,
			Optional:         true,
			Description:      "Where in the generated VCL the logging call should be placed.",
			ValidateDiagFunc: validateLoggingPlacement(),
		}
	}

	return &schema.Schema{
		Type:     schema.TypeSet,
		Optional: true,
		Elem: &schema.Resource{
			Schema: blockAttributes,
		},
	}
}

func (h *S3LoggingServiceAttributeHandler) Create(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts, err := h.buildCreate(resource, d.Id(), serviceVersion)
	if err != nil {
		return err
	}

	err = createS3(conn, opts)
	if err != nil {
		return err
	}
	return nil
}

func (h *S3LoggingServiceAttributeHandler) Read(_ context.Context, d *schema.ResourceData, _ map[string]interface{}, serviceVersion int, conn *gofastly.Client) error {
	// Refresh S3.
	log.Printf("[DEBUG] Refreshing S3 Logging for (%s)", d.Id())
	s3List, err := conn.ListS3s(&gofastly.ListS3sInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up S3 Logging for (%s), version (%v): %s", d.Id(), serviceVersion, err)
	}

	sl := flattenS3s(s3List)

	for _, element := range sl {
		element = h.pruneVCLLoggingAttributes(element)
	}

	if err := d.Set(h.GetKey(), sl); err != nil {
		log.Printf("[WARN] Error setting S3 Logging for (%s): %s", d.Id(), err)
	}
	return nil
}

func (h *S3LoggingServiceAttributeHandler) Update(_ context.Context, d *schema.ResourceData, resource, modified map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.UpdateS3Input{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
		Name:           resource["name"].(string),
	}

	// NOTE: where we transition between interface{} we lose the ability to
	// infer the underlying type being either a uint vs an int. This
	// materializes as a panic (yay) and so it's only at runtime we discover
	// this and so we've updated the below code to convert the type asserted
	// int into a uint before passing the value to gofastly.Uint().
	if v, ok := modified["bucket_name"]; ok {
		opts.BucketName = gofastly.String(v.(string))
	}
	if v, ok := modified["domain"]; ok {
		opts.Domain = gofastly.String(v.(string))
	}
	if v, ok := modified["s3_access_key"]; ok {
		opts.AccessKey = gofastly.String(v.(string))
	}
	if v, ok := modified["s3_secret_key"]; ok {
		opts.SecretKey = gofastly.String(v.(string))
	}
	if v, ok := modified["s3_iam_role"]; ok {
		opts.IAMRole = gofastly.String(v.(string))
	}
	if v, ok := modified["path"]; ok {
		opts.Path = gofastly.String(v.(string))
	}
	if v, ok := modified["period"]; ok {
		opts.Period = gofastly.Uint(uint(v.(int)))
	}
	if v, ok := modified["compression_codec"]; ok {
		opts.CompressionCodec = gofastly.String(v.(string))
	}
	if v, ok := modified["gzip_level"]; ok {
		opts.GzipLevel = gofastly.Uint(uint(v.(int)))
	}
	if v, ok := modified["format"]; ok {
		opts.Format = gofastly.String(v.(string))
	}
	if v, ok := modified["format_version"]; ok {
		opts.FormatVersion = gofastly.Uint(uint(v.(int)))
	}
	if v, ok := modified["response_condition"]; ok {
		opts.ResponseCondition = gofastly.String(v.(string))
	}
	if v, ok := modified["message_type"]; ok {
		opts.MessageType = gofastly.String(v.(string))
	}
	if v, ok := modified["timestamp_format"]; ok {
		opts.TimestampFormat = gofastly.String(v.(string))
	}
	if v, ok := modified["redundancy"]; ok {
		opts.Redundancy = gofastly.S3RedundancyPtr(gofastly.S3Redundancy(v.(string)))
	}
	if v, ok := modified["placement"]; ok {
		opts.Placement = gofastly.String(v.(string))
	}
	if v, ok := modified["public_key"]; ok {
		opts.PublicKey = gofastly.String(v.(string))
	}
	if v, ok := modified["server_side_encryption_kms_key_id"]; ok {
		opts.ServerSideEncryptionKMSKeyID = gofastly.String(v.(string))
	}
	if v, ok := modified["server_side_encryption"]; ok {
		opts.ServerSideEncryption = gofastly.S3ServerSideEncryptionPtr(gofastly.S3ServerSideEncryption(v.(string)))
	}
	if v, ok := modified["acl"]; ok {
		opts.ACL = gofastly.S3AccessControlListPtr(gofastly.S3AccessControlList(v.(string)))
	}

	log.Printf("[DEBUG] Update S3 Opts: %#v", opts)
	_, err := conn.UpdateS3(&opts)
	if err != nil {
		return err
	}
	return nil
}

func (h *S3LoggingServiceAttributeHandler) Delete(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := h.buildDelete(resource, d.Id(), serviceVersion)
	err := deleteS3(conn, opts)
	if err != nil {
		return err
	}
	return nil
}

func createS3(conn *gofastly.Client, i *gofastly.CreateS3Input) error {
	_, err := conn.CreateS3(i)
	return err
}

func deleteS3(conn *gofastly.Client, i *gofastly.DeleteS3Input) error {
	log.Printf("[DEBUG] Fastly S3 Logging removal opts: %#v", i)

	err := conn.DeleteS3(i)

	errRes, ok := err.(*gofastly.HTTPError)
	if !ok {
		return err
	}

	// 404 response codes don't result in an error propagating because a 404 could
	// indicate that a resource was deleted elsewhere.
	if !errRes.IsNotFound() {
		return err
	}

	return nil
}

func flattenS3s(s3List []*gofastly.S3) []map[string]interface{} {
	var sl []map[string]interface{}
	for _, s := range s3List {
		// Convert S3s to a map for saving to state.
		ns := map[string]interface{}{
			"name":                              s.Name,
			"bucket_name":                       s.BucketName,
			"s3_access_key":                     s.AccessKey,
			"s3_secret_key":                     s.SecretKey,
			"s3_iam_role":                       s.IAMRole,
			"path":                              s.Path,
			"period":                            s.Period,
			"domain":                            s.Domain,
			"gzip_level":                        s.GzipLevel,
			"format":                            s.Format,
			"format_version":                    s.FormatVersion,
			"timestamp_format":                  s.TimestampFormat,
			"redundancy":                        s.Redundancy,
			"response_condition":                s.ResponseCondition,
			"message_type":                      s.MessageType,
			"public_key":                        s.PublicKey,
			"placement":                         s.Placement,
			"server_side_encryption":            s.ServerSideEncryption,
			"server_side_encryption_kms_key_id": s.ServerSideEncryptionKMSKeyID,
			"compression_codec":                 s.CompressionCodec,
			"acl":                               s.ACL,
		}

		// Prune any empty values that come from the default string value in structs.
		for k, v := range ns {
			if v == "" {
				delete(ns, k)
			}
		}

		sl = append(sl, ns)
	}

	return sl
}

func (h *S3LoggingServiceAttributeHandler) buildCreate(s3Map interface{}, serviceID string, serviceVersion int) (*gofastly.CreateS3Input, error) {
	df := s3Map.(map[string]interface{})

	var vla = h.getVCLLoggingAttributes(df)
	opts := gofastly.CreateS3Input{
		ServiceID:                    serviceID,
		ServiceVersion:               serviceVersion,
		Name:                         df["name"].(string),
		BucketName:                   df["bucket_name"].(string),
		AccessKey:                    df["s3_access_key"].(string),
		SecretKey:                    df["s3_secret_key"].(string),
		IAMRole:                      df["s3_iam_role"].(string),
		Period:                       uint(df["period"].(int)),
		GzipLevel:                    uint(df["gzip_level"].(int)),
		Domain:                       df["domain"].(string),
		Path:                         df["path"].(string),
		TimestampFormat:              df["timestamp_format"].(string),
		MessageType:                  df["message_type"].(string),
		PublicKey:                    df["public_key"].(string),
		ServerSideEncryptionKMSKeyID: df["server_side_encryption_kms_key_id"].(string),
		CompressionCodec:             df["compression_codec"].(string),
		Format:                       vla.format,
		FormatVersion:                uintOrDefault(vla.formatVersion),
		ResponseCondition:            vla.responseCondition,
		Placement:                    vla.placement,
		Redundancy:                   gofastly.S3Redundancy(df["redundancy"].(string)),
		ACL:                          gofastly.S3AccessControlList(df["acl"].(string)),
	}

	encryption := df["server_side_encryption"].(string)
	switch encryption {
	case string(gofastly.S3ServerSideEncryptionAES):
		opts.ServerSideEncryption = gofastly.S3ServerSideEncryptionAES
	case string(gofastly.S3ServerSideEncryptionKMS):
		opts.ServerSideEncryption = gofastly.S3ServerSideEncryptionKMS
	}

	return &opts, nil
}

func (h *S3LoggingServiceAttributeHandler) buildDelete(s3Map interface{}, serviceID string, serviceVersion int) *gofastly.DeleteS3Input {
	df := s3Map.(map[string]interface{})

	return &gofastly.DeleteS3Input{
		ServiceID:      serviceID,
		ServiceVersion: serviceVersion,
		Name:           df["name"].(string),
	}
}
