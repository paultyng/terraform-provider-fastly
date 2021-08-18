---
layout: "fastly"
page_title: "Fastly: service_v1"
sidebar_current: "docs-fastly-resource-service-v1"
description: |-
  Provides an Fastly Service
---

# fastly_service_v1

Provides a Fastly Service, representing the configuration for a website, app,
API, or anything else to be served through Fastly. A Service encompasses Domains
and Backends.

The Service resource requires a domain name that is correctly set up to direct
traffic to the Fastly service. See Fastly's guide on [Adding CNAME Records][fastly-cname]
on their documentation site for guidance.

## Example Usage

Basic usage:

```hcl
resource "fastly_service_v1" "demo" {
  name = "demofastly"

  domain {
    name    = "demo.notexample.com"
    comment = "demo"
  }

  backend {
    address = "127.0.0.1"
    name    = "localhost"
    port    = 80
  }

  force_destroy = true
}
```

Basic usage with an Amazon S3 Website and that removes the `x-amz-request-id` header:

```hcl
resource "fastly_service_v1" "demo" {
  name = "demofastly"

  domain {
    name    = "demo.notexample.com"
    comment = "demo"
  }

  backend {
    address = "demo.notexample.com.s3-website-us-west-2.amazonaws.com"
    name    = "AWS S3 hosting"
    port    = 80
    override_host = "demo.notexample.com.s3-website-us-west-2.amazonaws.com"
  }

  header {
    destination = "http.x-amz-request-id"
    type        = "cache"
    action      = "delete"
    name        = "remove x-amz-request-id"
  }

  gzip {
    name          = "file extensions and content types"
    extensions    = ["css", "js"]
    content_types = ["text/html", "text/css"]
  }

  force_destroy = true
}

resource "aws_s3_bucket" "website" {
  bucket = "demo.notexample.com"
  acl    = "public-read"

  website {
    index_document = "index.html"
    error_document = "error.html"
  }
}
```

Basic usage with [custom
VCL](https://docs.fastly.com/vcl/custom-vcl/uploading-custom-vcl/):

```hcl
resource "fastly_service_v1" "demo" {
  name = "demofastly"

  domain {
    name    = "demo.notexample.com"
    comment = "demo"
  }

  backend {
    address = "127.0.0.1"
    name    = "localhost"
    port    = 80
  }

  force_destroy = true

  vcl {
    name    = "my_custom_main_vcl"
    content = "${file("${path.module}/my_custom_main.vcl")}"
    main    = true
  }

  vcl {
    name    = "my_custom_library_vcl"
    content = "${file("${path.module}/my_custom_library.vcl")}"
  }
}
```

Basic usage with [custom Director](https://developer.fastly.com/reference/api/load-balancing/directors/director/):

```hcl
resource "fastly_service_v1" "demo" {
  name = "demofastly"

  domain {
    name    = "demo.notexample.com"
    comment = "demo"
  }

  backend {
    address = "127.0.0.1"
    name    = "origin1"
    port    = 80
  }

  backend {
    address = "127.0.0.2"
    name    = "origin2"
    port    = 80
  }

  director {
    name = "mydirector"
    quorum = 0
    type = 3
    backends = [ "origin1", "origin2" ]
  }

  force_destroy = true
}
```

-> **Note:** The following example is only available from 0.20.0 of the Fastly Terraform provider.

Basic usage with [Web Application Firewall](https://developer.fastly.com/reference/api/waf/):

```hcl
resource "fastly_service_v1" "demo" {
  name = "demofastly"

  domain {
    name    = "example.com"
    comment = "demo"
  }

  backend {
    address = "127.0.0.1"
    name    = "origin1"
    port    = 80
  }

  condition {
    name      = "WAF_Prefetch"
    type      = "PREFETCH"
    statement = "req.backend.is_origin"
  }

  # This condition will always be false
  # adding it to the response object created below
  # prevents Fastly from returning a 403 on all of your traffic.
  condition {
    name      = "WAF_always_false"
    statement = "false"
    type      = "REQUEST"
  }

  response_object {
    name              = "WAF_Response"
    status            = "403"
    response          = "Forbidden"
    content_type      = "text/html"
    content           = "<html><body>Forbidden</body></html>"
    request_condition = "WAF_always_false"
  }

  waf {
    prefetch_condition = "WAF_Prefetch"
    response_object    = "WAF_Response"
  }

  force_destroy = true
}
```

-> **Note:** For an AWS S3 Bucket, the Backend address is
`<domain>.s3-website-<region>.amazonaws.com`. The `override_host` attribute
should be set to `<bucket_name>.s3-website-<region>.amazonaws.com` in the `backend` block. See the
Fastly documentation on [Amazon S3][fastly-s3].

[fastly-s3]: https://docs.fastly.com/en/guides/amazon-s3
[fastly-cname]: https://docs.fastly.com/en/guides/adding-cname-records
[fastly-conditionals]: https://docs.fastly.com/en/guides/using-conditions
[fastly-sumologic]: https://developer.fastly.com/reference/api/logging/sumologic/
[fastly-gcs]: https://developer.fastly.com/reference/api/logging/gcs/

## Import

Fastly Services can be imported using their service ID, e.g.

```
$ terraform import fastly_service_v1.demo xxxxxxxxxxxxxxxxxxxx
```


By default, either the active version will be imported, or the latest version if no version is active.
Alternatively, a specific version of the service can be selected by appending an `@` followed by the version number to the service ID, e.g.

```
$ terraform import fastly_service_v1.demo xxxxxxxxxxxxxxxxxxxx@2
```


<!-- schema generated by tfplugindocs -->
## Schema

### Required

- **domain** (Block Set, Min: 1) A set of Domain names to serve as entry points for your Service (see [below for nested schema](#nestedblock--domain))
- **name** (String) The unique name for the Service to create

### Optional

- **acl** (Block Set) (see [below for nested schema](#nestedblock--acl))
- **activate** (Boolean) Conditionally prevents the Service from being activated. The apply step will continue to create a new draft version but will not activate it if this is set to `false`. Default `true`
- **backend** (Block Set) (see [below for nested schema](#nestedblock--backend))
- **bigquerylogging** (Block Set) (see [below for nested schema](#nestedblock--bigquerylogging))
- **blobstoragelogging** (Block Set) (see [below for nested schema](#nestedblock--blobstoragelogging))
- **cache_setting** (Block Set) (see [below for nested schema](#nestedblock--cache_setting))
- **comment** (String) Description field for the service. Default `Managed by Terraform`
- **condition** (Block Set) (see [below for nested schema](#nestedblock--condition))
- **default_host** (String) The default hostname
- **default_ttl** (Number) The default Time-to-live (TTL) for requests
- **dictionary** (Block Set) (see [below for nested schema](#nestedblock--dictionary))
- **director** (Block Set) (see [below for nested schema](#nestedblock--director))
- **dynamicsnippet** (Block Set) (see [below for nested schema](#nestedblock--dynamicsnippet))
- **force_destroy** (Boolean) Services that are active cannot be destroyed. In order to destroy the Service, set `force_destroy` to `true`. Default `false`
- **gcslogging** (Block Set) (see [below for nested schema](#nestedblock--gcslogging))
- **gzip** (Block Set) (see [below for nested schema](#nestedblock--gzip))
- **header** (Block Set) (see [below for nested schema](#nestedblock--header))
- **healthcheck** (Block Set) (see [below for nested schema](#nestedblock--healthcheck))
- **httpslogging** (Block Set) (see [below for nested schema](#nestedblock--httpslogging))
- **id** (String) The ID of this resource.
- **logentries** (Block Set) (see [below for nested schema](#nestedblock--logentries))
- **logging_cloudfiles** (Block Set) (see [below for nested schema](#nestedblock--logging_cloudfiles))
- **logging_datadog** (Block Set) (see [below for nested schema](#nestedblock--logging_datadog))
- **logging_digitalocean** (Block Set) (see [below for nested schema](#nestedblock--logging_digitalocean))
- **logging_elasticsearch** (Block Set) (see [below for nested schema](#nestedblock--logging_elasticsearch))
- **logging_ftp** (Block Set) (see [below for nested schema](#nestedblock--logging_ftp))
- **logging_googlepubsub** (Block Set) (see [below for nested schema](#nestedblock--logging_googlepubsub))
- **logging_heroku** (Block Set) (see [below for nested schema](#nestedblock--logging_heroku))
- **logging_honeycomb** (Block Set) (see [below for nested schema](#nestedblock--logging_honeycomb))
- **logging_kafka** (Block Set) (see [below for nested schema](#nestedblock--logging_kafka))
- **logging_kinesis** (Block Set) (see [below for nested schema](#nestedblock--logging_kinesis))
- **logging_loggly** (Block Set) (see [below for nested schema](#nestedblock--logging_loggly))
- **logging_logshuttle** (Block Set) (see [below for nested schema](#nestedblock--logging_logshuttle))
- **logging_newrelic** (Block Set) (see [below for nested schema](#nestedblock--logging_newrelic))
- **logging_openstack** (Block Set) (see [below for nested schema](#nestedblock--logging_openstack))
- **logging_scalyr** (Block Set) (see [below for nested schema](#nestedblock--logging_scalyr))
- **logging_sftp** (Block Set) (see [below for nested schema](#nestedblock--logging_sftp))
- **papertrail** (Block Set) (see [below for nested schema](#nestedblock--papertrail))
- **request_setting** (Block Set) (see [below for nested schema](#nestedblock--request_setting))
- **response_object** (Block Set) (see [below for nested schema](#nestedblock--response_object))
- **s3logging** (Block Set) (see [below for nested schema](#nestedblock--s3logging))
- **snippet** (Block Set) (see [below for nested schema](#nestedblock--snippet))
- **splunk** (Block Set) (see [below for nested schema](#nestedblock--splunk))
- **sumologic** (Block Set) (see [below for nested schema](#nestedblock--sumologic))
- **syslog** (Block Set) (see [below for nested schema](#nestedblock--syslog))
- **vcl** (Block Set) (see [below for nested schema](#nestedblock--vcl))
- **version_comment** (String) Description field for the version
- **waf** (Block List, Max: 1) (see [below for nested schema](#nestedblock--waf))

### Read-Only

- **active_version** (Number) The currently active version of your Fastly Service
- **cloned_version** (Number) The latest cloned version by the provider

<a id="nestedblock--domain"></a>
### Nested Schema for `domain`

Required:

- **name** (String) The domain that this Service will respond to. It is important to note that changing this attribute will delete and recreate the resource.

Optional:

- **comment** (String) An optional comment about the Domain.


<a id="nestedblock--acl"></a>
### Nested Schema for `acl`

Required:

- **name** (String) A unique name to identify this ACL. It is important to note that changing this attribute will delete and recreate the ACL, and discard the current items in the ACL

Optional:

- **force_destroy** (Boolean) Allow the ACL to be deleted, even if it contains entries. Defaults to false.

Read-Only:

- **acl_id** (String) The ID of the ACL


<a id="nestedblock--backend"></a>
### Nested Schema for `backend`

Required:

- **address** (String) An IPv4, hostname, or IPv6 address for the Backend
- **name** (String) Name for this Backend. Must be unique to this Service. It is important to note that changing this attribute will delete and recreate the resource

Optional:

- **auto_loadbalance** (Boolean) Denotes if this Backend should be included in the pool of backends that requests are load balanced against. Default `false`
- **between_bytes_timeout** (Number) How long to wait between bytes in milliseconds. Default `10000`
- **connect_timeout** (Number) How long to wait for a timeout in milliseconds. Default `1000`
- **error_threshold** (Number) Number of errors to allow before the Backend is marked as down. Default `0`
- **first_byte_timeout** (Number) How long to wait for the first bytes in milliseconds. Default `15000`
- **healthcheck** (String) Name of a defined `healthcheck` to assign to this backend
- **max_conn** (Number) Maximum number of connections for this Backend. Default `200`
- **max_tls_version** (String) Maximum allowed TLS version on SSL connections to this backend.
- **min_tls_version** (String) Minimum allowed TLS version on SSL connections to this backend.
- **override_host** (String) The hostname to override the Host header
- **port** (Number) The port number on which the Backend responds. Default `80`
- **request_condition** (String) Name of a condition, which if met, will select this backend during a request.
- **shield** (String) The POP of the shield designated to reduce inbound load. Valid values for `shield` are included in the `GET /datacenters` API response
- **ssl_ca_cert** (String) CA certificate attached to origin.
- **ssl_cert_hostname** (String) Overrides ssl_hostname, but only for cert verification. Does not affect SNI at all
- **ssl_check_cert** (Boolean) Be strict about checking SSL certs. Default `true`
- **ssl_ciphers** (String) Comma separated list of OpenSSL Ciphers to try when negotiating to the backend
- **ssl_client_cert** (String, Sensitive) Client certificate attached to origin. Used when connecting to the backend
- **ssl_client_key** (String, Sensitive) Client key attached to origin. Used when connecting to the backend
- **ssl_hostname** (String, Deprecated) Used for both SNI during the TLS handshake and to validate the cert
- **ssl_sni_hostname** (String) Overrides ssl_hostname, but only for SNI in the handshake. Does not affect cert validation at all
- **use_ssl** (Boolean) Whether or not to use SSL to reach the Backend. Default `false`
- **weight** (Number) The [portion of traffic](https://docs.fastly.com/en/guides/load-balancing-configuration#how-weight-affects-load-balancing) to send to this Backend. Each Backend receives weight / total of the traffic. Default `100`


<a id="nestedblock--bigquerylogging"></a>
### Nested Schema for `bigquerylogging`

Required:

- **dataset** (String) The ID of your BigQuery dataset
- **name** (String) A unique name to identify this BigQuery logging endpoint. It is important to note that changing this attribute will delete and recreate the resource
- **project_id** (String) The ID of your GCP project
- **table** (String) The ID of your BigQuery table

Optional:

- **email** (String, Sensitive) The email for the service account with write access to your BigQuery dataset. If not provided, this will be pulled from a `FASTLY_BQ_EMAIL` environment variable
- **format** (String) The logging format desired.
- **placement** (String) Where in the generated VCL the logging call should be placed.
- **response_condition** (String) Name of a condition to apply this logging.
- **secret_key** (String, Sensitive) The secret key associated with the service account that has write access to your BigQuery table. If not provided, this will be pulled from the `FASTLY_BQ_SECRET_KEY` environment variable. Typical format for this is a private key in a string with newlines
- **template** (String) BigQuery table name suffix template


<a id="nestedblock--blobstoragelogging"></a>
### Nested Schema for `blobstoragelogging`

Required:

- **account_name** (String) The unique Azure Blob Storage namespace in which your data objects are stored
- **container** (String) The name of the Azure Blob Storage container in which to store logs
- **name** (String) A unique name to identify the Azure Blob Storage endpoint. It is important to note that changing this attribute will delete and recreate the resource

Optional:

- **compression_codec** (String) The codec used for compression of your logs. Valid values are zstd, snappy, and gzip. If the specified codec is "gzip", gzip_level will default to 3. To specify a different level, leave compression_codec blank and explicitly set the level using gzip_level. Specifying both compression_codec and gzip_level in the same API request will result in an error.
- **file_max_bytes** (Number) Maximum size of an uploaded log file, if non-zero.
- **format** (String) Apache-style string or VCL variables to use for log formatting (default: `%h %l %u %t "%r" %>s %b`)
- **format_version** (Number) The version of the custom logging format used for the configured endpoint. Can be either 1 or 2. (default: 2)
- **gzip_level** (Number) Level of Gzip compression from `0-9`. `0` means no compression. `1` is the fastest and the least compressed version, `9` is the slowest and the most compressed version. Default `0`
- **message_type** (String) How the message should be formatted. Can be either `classic`, `loggly`, `logplex` or `blank`. Default `classic`
- **path** (String) The path to upload logs to. Must end with a trailing slash. If this field is left empty, the files will be saved in the container's root path
- **period** (Number) How frequently the logs should be transferred in seconds. Default `3600`
- **placement** (String) Where in the generated VCL the logging call should be placed
- **public_key** (String) A PGP public key that Fastly will use to encrypt your log files before writing them to disk
- **response_condition** (String) The name of the condition to apply
- **sas_token** (String, Sensitive) The Azure shared access signature providing write access to the blob service objects. Be sure to update your token before it expires or the logging functionality will not work
- **timestamp_format** (String) `strftime` specified timestamp formatting. Default `%Y-%m-%dT%H:%M:%S.000`


<a id="nestedblock--cache_setting"></a>
### Nested Schema for `cache_setting`

Required:

- **name** (String) Unique name for this Cache Setting. It is important to note that changing this attribute will delete and recreate the resource

Optional:

- **action** (String) One of cache, pass, or restart, as defined on Fastly's documentation under "[Caching action descriptions](https://docs.fastly.com/en/guides/controlling-caching#caching-action-descriptions)"
- **cache_condition** (String) Name of already defined `condition` used to test whether this settings object should be used. This `condition` must be of type `CACHE`
- **stale_ttl** (Number) Max "Time To Live" for stale (unreachable) objects
- **ttl** (Number) The Time-To-Live (TTL) for the object


<a id="nestedblock--condition"></a>
### Nested Schema for `condition`

Required:

- **name** (String) The unique name for the condition. It is important to note that changing this attribute will delete and recreate the resource
- **statement** (String) The statement used to determine if the condition is met
- **type** (String) Type of condition, either `REQUEST` (req), `RESPONSE` (req, resp), or `CACHE` (req, beresp)

Optional:

- **priority** (Number) A number used to determine the order in which multiple conditions execute. Lower numbers execute first. Default `10`


<a id="nestedblock--dictionary"></a>
### Nested Schema for `dictionary`

Required:

- **name** (String) A unique name to identify this dictionary. It is important to note that changing this attribute will delete and recreate the dictionary, and discard the current items in the dictionary

Optional:

- **force_destroy** (Boolean) Allow the dictionary to be deleted, even if it contains entries. Defaults to false.
- **write_only** (Boolean) If `true`, the dictionary is a private dictionary, and items are not readable in the UI or via API. Default is `false`. It is important to note that changing this attribute will delete and recreate the dictionary, and discard the current items in the dictionary. Using a write-only/private dictionary should only be done if the items are managed outside of Terraform

Read-Only:

- **dictionary_id** (String) The ID of the dictionary


<a id="nestedblock--director"></a>
### Nested Schema for `director`

Required:

- **backends** (Set of String) Names of defined backends to map the director to. Example: `[ "origin1", "origin2" ]`
- **name** (String) Unique name for this Director. It is important to note that changing this attribute will delete and recreate the resource

Optional:

- **capacity** (Number) Load balancing weight for the backends. Default `100`
- **comment** (String) An optional comment about the Director
- **quorum** (Number) Percentage of capacity that needs to be up for the director itself to be considered up. Default `75`
- **retries** (Number) How many backends to search if it fails. Default `5`
- **shield** (String) Selected POP to serve as a "shield" for backends. Valid values for `shield` are included in the [`GET /datacenters`](https://developer.fastly.com/reference/api/utils/datacenter/) API response
- **type** (Number) Type of load balance group to use. Integer, 1 to 4. Values: `1` (random), `3` (hash), `4` (client). Default `1`


<a id="nestedblock--dynamicsnippet"></a>
### Nested Schema for `dynamicsnippet`

Required:

- **name** (String) A name that is unique across "regular" and "dynamic" VCL Snippet configuration blocks. It is important to note that changing this attribute will delete and recreate the resource
- **type** (String) The location in generated VCL where the snippet should be placed (can be one of `init`, `recv`, `hit`, `miss`, `pass`, `fetch`, `error`, `deliver`, `log` or `none`)

Optional:

- **priority** (Number) Priority determines the ordering for multiple snippets. Lower numbers execute first. Defaults to `100`

Read-Only:

- **snippet_id** (String) The ID of the dynamic snippet


<a id="nestedblock--gcslogging"></a>
### Nested Schema for `gcslogging`

Required:

- **bucket_name** (String) The name of the bucket in which to store the logs
- **name** (String) A unique name to identify this GCS endpoint. It is important to note that changing this attribute will delete and recreate the resource

Optional:

- **compression_codec** (String) The codec used for compression of your logs. Valid values are zstd, snappy, and gzip. If the specified codec is "gzip", gzip_level will default to 3. To specify a different level, leave compression_codec blank and explicitly set the level using gzip_level. Specifying both compression_codec and gzip_level in the same API request will result in an error.
- **email** (String) The email address associated with the target GCS bucket on your account. You may optionally provide this secret via an environment variable, `FASTLY_GCS_EMAIL`
- **format** (String) Apache-style string or VCL variables to use for log formatting
- **gzip_level** (Number) Level of Gzip compression, from `0-9`. `0` is no compression. `1` is fastest and least compressed, `9` is slowest and most compressed. Default `0`
- **message_type** (String) How the message should be formatted; one of: `classic`, `loggly`, `logplex` or `blank`. Default `classic`. [Fastly Documentation](https://developer.fastly.com/reference/api/logging/gcs/)
- **path** (String) Path to store the files. Must end with a trailing slash. If this field is left empty, the files will be saved in the bucket's root path
- **period** (Number) How frequently the logs should be transferred, in seconds (Default 3600)
- **placement** (String) Where in the generated VCL the logging call should be placed.
- **response_condition** (String) Name of a condition to apply this logging.
- **secret_key** (String, Sensitive) The secret key associated with the target gcs bucket on your account. You may optionally provide this secret via an environment variable, `FASTLY_GCS_SECRET_KEY`. A typical format for the key is PEM format, containing actual newline characters where required
- **timestamp_format** (String) specified timestamp formatting (default `%Y-%m-%dT%H:%M:%S.000`)


<a id="nestedblock--gzip"></a>
### Nested Schema for `gzip`

Required:

- **name** (String) A name to refer to this gzip condition. It is important to note that changing this attribute will delete and recreate the resource

Optional:

- **cache_condition** (String) Name of already defined `condition` controlling when this gzip configuration applies. This `condition` must be of type `CACHE`. For detailed information about Conditionals, see [Fastly's Documentation on Conditionals](https://docs.fastly.com/en/guides/using-conditions)
- **content_types** (Set of String) The content-type for each type of content you wish to have dynamically gzip'ed. Example: `["text/html", "text/css"]`
- **extensions** (Set of String) File extensions for each file type to dynamically gzip. Example: `["css", "js"]`


<a id="nestedblock--header"></a>
### Nested Schema for `header`

Required:

- **action** (String) The Header manipulation action to take; must be one of `set`, `append`, `delete`, `regex`, or `regex_repeat`
- **destination** (String) The name of the header that is going to be affected by the Action
- **name** (String) Unique name for this header attribute. It is important to note that changing this attribute will delete and recreate the resource
- **type** (String) The Request type on which to apply the selected Action; must be one of `request`, `fetch`, `cache` or `response`

Optional:

- **cache_condition** (String) Name of already defined `condition` to apply. This `condition` must be of type `CACHE`
- **ignore_if_set** (Boolean) Don't add the header if it is already. (Only applies to `set` action.). Default `false`
- **priority** (Number) Lower priorities execute first. Default: `100`
- **regex** (String) Regular expression to use (Only applies to `regex` and `regex_repeat` actions.)
- **request_condition** (String) Name of already defined `condition` to apply. This `condition` must be of type `REQUEST`
- **response_condition** (String) Name of already defined `condition` to apply. This `condition` must be of type `RESPONSE`. For detailed information about Conditionals, see [Fastly's Documentation on Conditionals](https://docs.fastly.com/en/guides/using-conditions)
- **source** (String) Variable to be used as a source for the header content (Does not apply to `delete` action.)
- **substitution** (String) Value to substitute in place of regular expression. (Only applies to `regex` and `regex_repeat`.)


<a id="nestedblock--healthcheck"></a>
### Nested Schema for `healthcheck`

Required:

- **host** (String) The Host header to send for this Healthcheck
- **name** (String) A unique name to identify this Healthcheck. It is important to note that changing this attribute will delete and recreate the resource
- **path** (String) The path to check

Optional:

- **check_interval** (Number) How often to run the Healthcheck in milliseconds. Default `5000`
- **expected_response** (Number) The status code expected from the host. Default `200`
- **http_version** (String) Whether to use version 1.0 or 1.1 HTTP. Default `1.1`
- **initial** (Number) When loading a config, the initial number of probes to be seen as OK. Default `3`
- **method** (String) Which HTTP method to use. Default `HEAD`
- **threshold** (Number) How many Healthchecks must succeed to be considered healthy. Default `3`
- **timeout** (Number) Timeout in milliseconds. Default `500`
- **window** (Number) The number of most recent Healthcheck queries to keep for this Healthcheck. Default `5`


<a id="nestedblock--httpslogging"></a>
### Nested Schema for `httpslogging`

Required:

- **name** (String) The unique name of the HTTPS logging endpoint. It is important to note that changing this attribute will delete and recreate the resource
- **url** (String) URL that log data will be sent to. Must use the https protocol

Optional:

- **content_type** (String) Value of the `Content-Type` header sent with the request
- **format** (String) Apache-style string or VCL variables to use for log formatting.
- **format_version** (Number) The version of the custom logging format used for the configured endpoint. Can be either 1 or 2. (default: 2)
- **header_name** (String) Custom header sent with the request
- **header_value** (String) Value of the custom header sent with the request
- **json_format** (String) Formats log entries as JSON. Can be either disabled (`0`), array of json (`1`), or newline delimited json (`2`)
- **message_type** (String) How the message should be formatted; one of: `classic`, `loggly`, `logplex` or `blank`. Default `blank`
- **method** (String) HTTP method used for request. Can be either `POST` or `PUT`. Default `POST`
- **placement** (String) Where in the generated VCL the logging call should be placed
- **request_max_bytes** (Number) The maximum number of bytes sent in one request
- **request_max_entries** (Number) The maximum number of logs sent in one request
- **response_condition** (String) The name of the condition to apply
- **tls_ca_cert** (String) A secure certificate to authenticate the server with. Must be in PEM format
- **tls_client_cert** (String) The client certificate used to make authenticated requests. Must be in PEM format
- **tls_client_key** (String, Sensitive) The client private key used to make authenticated requests. Must be in PEM format
- **tls_hostname** (String) Used during the TLS handshake to validate the certificate


<a id="nestedblock--logentries"></a>
### Nested Schema for `logentries`

Required:

- **name** (String) The unique name of the Logentries logging endpoint. It is important to note that changing this attribute will delete and recreate the resource
- **token** (String) Use token based authentication (https://logentries.com/doc/input-token/)

Optional:

- **format** (String) Apache-style string or VCL variables to use for log formatting
- **format_version** (Number) The version of the custom logging format used for the configured endpoint. Can be either 1 or 2. (Default: 1)
- **placement** (String) Where in the generated VCL the logging call should be placed.
- **port** (Number) The port number configured in Logentries
- **response_condition** (String) Name of blockAttributes condition to apply this logging.
- **use_tls** (Boolean) Whether to use TLS for secure logging


<a id="nestedblock--logging_cloudfiles"></a>
### Nested Schema for `logging_cloudfiles`

Required:

- **access_key** (String, Sensitive) Your Cloud File account access key
- **bucket_name** (String) The name of your Cloud Files container
- **name** (String) The unique name of the Rackspace Cloud Files logging endpoint. It is important to note that changing this attribute will delete and recreate the resource
- **user** (String) The username for your Cloud Files account

Optional:

- **compression_codec** (String) The codec used for compression of your logs. Valid values are zstd, snappy, and gzip. If the specified codec is "gzip", gzip_level will default to 3. To specify a different level, leave compression_codec blank and explicitly set the level using gzip_level. Specifying both compression_codec and gzip_level in the same API request will result in an error.
- **format** (String) Apache style log formatting.
- **format_version** (Number) The version of the custom logging format used for the configured endpoint. Can be either `1` or `2`. (default: `2`).
- **gzip_level** (Number) What level of GZIP encoding to have when dumping logs (default `0`, no compression)
- **message_type** (String) How the message should be formatted. One of: `classic` (default), `loggly`, `logplex` or `blank`
- **path** (String) The path to upload logs to
- **period** (Number) How frequently log files are finalized so they can be available for reading (in seconds, default `3600`)
- **placement** (String) Where in the generated VCL the logging call should be placed. Can be `none` or `waf_debug`.
- **public_key** (String) The PGP public key that Fastly will use to encrypt your log files before writing them to disk
- **region** (String) The region to stream logs to. One of: DFW (Dallas), ORD (Chicago), IAD (Northern Virginia), LON (London), SYD (Sydney), HKG (Hong Kong)
- **response_condition** (String) The name of an existing condition in the configured endpoint, or leave blank to always execute.
- **timestamp_format** (String) The `strftime` specified timestamp formatting (default `%Y-%m-%dT%H:%M:%S.000`)


<a id="nestedblock--logging_datadog"></a>
### Nested Schema for `logging_datadog`

Required:

- **name** (String) The unique name of the Datadog logging endpoint. It is important to note that changing this attribute will delete and recreate the resource
- **token** (String, Sensitive) The API key from your Datadog account

Optional:

- **format** (String) Apache-style string or VCL variables to use for log formatting.
- **format_version** (Number) The version of the custom logging format used for the configured endpoint. Can be either `1` or `2`. (default: `2`).
- **placement** (String) Where in the generated VCL the logging call should be placed.
- **region** (String) The region that log data will be sent to. One of `US` or `EU`. Defaults to `US` if undefined
- **response_condition** (String) The name of the condition to apply.


<a id="nestedblock--logging_digitalocean"></a>
### Nested Schema for `logging_digitalocean`

Required:

- **access_key** (String, Sensitive) Your DigitalOcean Spaces account access key
- **bucket_name** (String) The name of the DigitalOcean Space
- **name** (String) The unique name of the DigitalOcean Spaces logging endpoint. It is important to note that changing this attribute will delete and recreate the resource
- **secret_key** (String, Sensitive) Your DigitalOcean Spaces account secret key

Optional:

- **compression_codec** (String) The codec used for compression of your logs. Valid values are zstd, snappy, and gzip. If the specified codec is "gzip", gzip_level will default to 3. To specify a different level, leave compression_codec blank and explicitly set the level using gzip_level. Specifying both compression_codec and gzip_level in the same API request will result in an error.
- **domain** (String) The domain of the DigitalOcean Spaces endpoint (default `nyc3.digitaloceanspaces.com`)
- **format** (String) Apache style log formatting.
- **format_version** (Number) The version of the custom logging format used for the configured endpoint. Can be either `1` or `2`. (default: `2`).
- **gzip_level** (Number) What level of Gzip encoding to have when dumping logs (default `0`, no compression)
- **message_type** (String) How the message should be formatted. One of: `classic` (default), `loggly`, `logplex` or `blank`
- **path** (String) The path to upload logs to
- **period** (Number) How frequently log files are finalized so they can be available for reading (in seconds, default `3600`)
- **placement** (String) Where in the generated VCL the logging call should be placed. Can be `none` or `waf_debug`.
- **public_key** (String) A PGP public key that Fastly will use to encrypt your log files before writing them to disk
- **response_condition** (String) The name of an existing condition in the configured endpoint, or leave blank to always execute.
- **timestamp_format** (String) `strftime` specified timestamp formatting (default `%Y-%m-%dT%H:%M:%S.000`)


<a id="nestedblock--logging_elasticsearch"></a>
### Nested Schema for `logging_elasticsearch`

Required:

- **index** (String) The name of the Elasticsearch index to send documents (logs) to
- **name** (String) The unique name of the Elasticsearch logging endpoint. It is important to note that changing this attribute will delete and recreate the resource
- **url** (String) The Elasticsearch URL to stream logs to

Optional:

- **format** (String) Apache-style string or VCL variables to use for log formatting.
- **format_version** (Number) The version of the custom logging format used for the configured endpoint. Can be either 1 or 2. (default: 2).
- **password** (String, Sensitive) BasicAuth password for Elasticsearch
- **pipeline** (String) The ID of the Elasticsearch ingest pipeline to apply pre-process transformations to before indexing
- **placement** (String) Where in the generated VCL the logging call should be placed.
- **request_max_bytes** (Number) The maximum number of logs sent in one request. Defaults to `0` for unbounded
- **request_max_entries** (Number) The maximum number of bytes sent in one request. Defaults to `0` for unbounded
- **response_condition** (String) The name of the condition to apply
- **tls_ca_cert** (String) A secure certificate to authenticate the server with. Must be in PEM format
- **tls_client_cert** (String) The client certificate used to make authenticated requests. Must be in PEM format
- **tls_client_key** (String, Sensitive) The client private key used to make authenticated requests. Must be in PEM format
- **tls_hostname** (String) The hostname used to verify the server's certificate. It can either be the Common Name (CN) or a Subject Alternative Name (SAN)
- **user** (String) BasicAuth username for Elasticsearch


<a id="nestedblock--logging_ftp"></a>
### Nested Schema for `logging_ftp`

Required:

- **address** (String) The FTP address to stream logs to
- **name** (String) The unique name of the FTP logging endpoint. It is important to note that changing this attribute will delete and recreate the resource
- **password** (String, Sensitive) The password for the server (for anonymous use an email address)
- **path** (String) The path to upload log files to. If the path ends in `/` then it is treated as a directory
- **user** (String) The username for the server (can be `anonymous`)

Optional:

- **compression_codec** (String) The codec used for compression of your logs. Valid values are zstd, snappy, and gzip. If the specified codec is "gzip", gzip_level will default to 3. To specify a different level, leave compression_codec blank and explicitly set the level using gzip_level. Specifying both compression_codec and gzip_level in the same API request will result in an error.
- **format** (String) Apache-style string or VCL variables to use for log formatting.
- **format_version** (Number) The version of the custom logging format used for the configured endpoint. Can be either 1 or 2. (default: 2).
- **gzip_level** (Number) Gzip Compression level. Default `0`
- **message_type** (String) How the message should be formatted (default: `classic`)
- **period** (Number) How frequently the logs should be transferred, in seconds (Default `3600`)
- **placement** (String) Where in the generated VCL the logging call should be placed.
- **port** (Number) The port number. Default: `21`
- **public_key** (String) The PGP public key that Fastly will use to encrypt your log files before writing them to disk
- **response_condition** (String) The name of the condition to apply.
- **timestamp_format** (String) specified timestamp formatting (default `%Y-%m-%dT%H:%M:%S.000`)


<a id="nestedblock--logging_googlepubsub"></a>
### Nested Schema for `logging_googlepubsub`

Required:

- **name** (String) The unique name of the Google Cloud Pub/Sub logging endpoint. It is important to note that changing this attribute will delete and recreate the resource
- **project_id** (String) The ID of your Google Cloud Platform project
- **topic** (String) The Google Cloud Pub/Sub topic to which logs will be published

Optional:

- **format** (String) Apache style log formatting.
- **format_version** (Number) The version of the custom logging format used for the configured endpoint. Can be either 1 or 2. (default: 2).
- **placement** (String) Where in the generated VCL the logging call should be placed.
- **response_condition** (String) The name of an existing condition in the configured endpoint, or leave blank to always execute.
- **secret_key** (String, Sensitive) Your Google Cloud Platform account secret key. The `private_key` field in your service account authentication JSON. You may optionally provide this secret via an environment variable, `FASTLY_GOOGLE_PUBSUB_SECRET_KEY`.
- **user** (String) Your Google Cloud Platform service account email address. The `client_email` field in your service account authentication JSON. You may optionally provide this via an environment variable, `FASTLY_GOOGLE_PUBSUB_EMAIL`.


<a id="nestedblock--logging_heroku"></a>
### Nested Schema for `logging_heroku`

Required:

- **name** (String) The unique name of the Heroku logging endpoint. It is important to note that changing this attribute will delete and recreate the resource
- **token** (String, Sensitive) The token to use for authentication (https://www.heroku.com/docs/customer-token-authentication-token/)
- **url** (String) The URL to stream logs to

Optional:

- **format** (String) Apache-style string or VCL variables to use for log formatting.
- **format_version** (Number) The version of the custom logging format used for the configured endpoint. Can be either `1` or `2`. (default: `2`).
- **placement** (String) Where in the generated VCL the logging call should be placed. Can be `none` or `waf_debug`.
- **response_condition** (String) The name of an existing condition in the configured endpoint, or leave blank to always execute.


<a id="nestedblock--logging_honeycomb"></a>
### Nested Schema for `logging_honeycomb`

Required:

- **dataset** (String) The Honeycomb Dataset you want to log to
- **name** (String) The unique name of the Honeycomb logging endpoint. It is important to note that changing this attribute will delete and recreate the resource
- **token** (String, Sensitive) The Write Key from the Account page of your Honeycomb account

Optional:

- **format** (String) Apache style log formatting. Your log must produce valid JSON that Honeycomb can ingest.
- **format_version** (Number) The version of the custom logging format used for the configured endpoint. Can be either `1` or `2`. (default: `2`).
- **placement** (String) Where in the generated VCL the logging call should be placed. Can be `none` or `waf_debug`.
- **response_condition** (String) The name of an existing condition in the configured endpoint, or leave blank to always execute.


<a id="nestedblock--logging_kafka"></a>
### Nested Schema for `logging_kafka`

Required:

- **brokers** (String) A comma-separated list of IP addresses or hostnames of Kafka brokers
- **name** (String) The unique name of the Kafka logging endpoint. It is important to note that changing this attribute will delete and recreate the resource
- **topic** (String) The Kafka topic to send logs to

Optional:

- **auth_method** (String) SASL authentication method. One of: plain, scram-sha-256, scram-sha-512
- **compression_codec** (String) The codec used for compression of your logs. One of: `gzip`, `snappy`, `lz4`
- **format** (String) Apache style log formatting.
- **format_version** (Number) The version of the custom logging format used for the configured endpoint. Can be either 1 or 2. (default: 2).
- **parse_log_keyvals** (Boolean) Enables parsing of key=value tuples from the beginning of a logline, turning them into record headers
- **password** (String, Sensitive) SASL Pass
- **placement** (String) Where in the generated VCL the logging call should be placed.
- **request_max_bytes** (Number) Maximum size of log batch, if non-zero. Defaults to 0 for unbounded
- **required_acks** (String) The Number of acknowledgements a leader must receive before a write is considered successful. One of: `1` (default) One server needs to respond. `0` No servers need to respond. `-1`	Wait for all in-sync replicas to respond
- **response_condition** (String) The name of an existing condition in the configured endpoint, or leave blank to always execute.
- **tls_ca_cert** (String) A secure certificate to authenticate the server with. Must be in PEM format
- **tls_client_cert** (String) The client certificate used to make authenticated requests. Must be in PEM format
- **tls_client_key** (String, Sensitive) The client private key used to make authenticated requests. Must be in PEM format
- **tls_hostname** (String) The hostname used to verify the server's certificate. It can either be the Common Name or a Subject Alternative Name (SAN)
- **use_tls** (Boolean) Whether to use TLS for secure logging. Can be either `true` or `false`
- **user** (String) SASL User


<a id="nestedblock--logging_kinesis"></a>
### Nested Schema for `logging_kinesis`

Required:

- **name** (String) The unique name of the Kinesis logging endpoint. It is important to note that changing this attribute will delete and recreate the resource
- **topic** (String) The Kinesis stream name

Optional:

- **access_key** (String, Sensitive) The AWS access key to be used to write to the stream
- **format** (String) Apache style log formatting.
- **format_version** (Number) The version of the custom logging format used for the configured endpoint. Can be either `1` or `2`. (default: `2`).
- **iam_role** (String) The Amazon Resource Name (ARN) for the IAM role granting Fastly access to Kinesis. Not required if `access_key` and `secret_key` are provided.
- **placement** (String) Where in the generated VCL the logging call should be placed. Can be `none` or `waf_debug`.
- **region** (String) The AWS region the stream resides in. (Default: `us-east-1`)
- **response_condition** (String) The name of an existing condition in the configured endpoint, or leave blank to always execute.
- **secret_key** (String, Sensitive) The AWS secret access key to authenticate with


<a id="nestedblock--logging_loggly"></a>
### Nested Schema for `logging_loggly`

Required:

- **name** (String) The unique name of the Loggly logging endpoint. It is important to note that changing this attribute will delete and recreate the resource
- **token** (String, Sensitive) The token to use for authentication (https://www.loggly.com/docs/customer-token-authentication-token/).

Optional:

- **format** (String) Apache-style string or VCL variables to use for log formatting.
- **format_version** (Number) The version of the custom logging format used for the configured endpoint. Can be either `1` or `2`. (default: `2`).
- **placement** (String) Where in the generated VCL the logging call should be placed. Can be `none` or `waf_debug`.
- **response_condition** (String) The name of an existing condition in the configured endpoint, or leave blank to always execute.


<a id="nestedblock--logging_logshuttle"></a>
### Nested Schema for `logging_logshuttle`

Required:

- **name** (String) The unique name of the Log Shuttle logging endpoint. It is important to note that changing this attribute will delete and recreate the resource
- **token** (String, Sensitive) The data authentication token associated with this endpoint
- **url** (String) Your Log Shuttle endpoint URL

Optional:

- **format** (String) Apache style log formatting.
- **format_version** (Number) The version of the custom logging format used for the configured endpoint. Can be either `1` or `2`. (default: `2`).
- **placement** (String) Where in the generated VCL the logging call should be placed. Can be `none` or `waf_debug`.
- **response_condition** (String) The name of an existing condition in the configured endpoint, or leave blank to always execute.


<a id="nestedblock--logging_newrelic"></a>
### Nested Schema for `logging_newrelic`

Required:

- **name** (String) The unique name of the New Relic logging endpoint. It is important to note that changing this attribute will delete and recreate the resource
- **token** (String, Sensitive) The Insert API key from the Account page of your New Relic account

Optional:

- **format** (String) Apache style log formatting. Your log must produce valid JSON that New Relic Logs can ingest.
- **format_version** (Number) The version of the custom logging format used for the configured endpoint. Can be either `1` or `2`. (default: `2`).
- **placement** (String) Where in the generated VCL the logging call should be placed.
- **response_condition** (String) The name of the condition to apply.


<a id="nestedblock--logging_openstack"></a>
### Nested Schema for `logging_openstack`

Required:

- **access_key** (String, Sensitive) Your OpenStack account access key
- **bucket_name** (String) The name of your OpenStack container
- **name** (String) The unique name of the OpenStack logging endpoint. It is important to note that changing this attribute will delete and recreate the resource
- **url** (String) Your OpenStack auth url
- **user** (String) The username for your OpenStack account

Optional:

- **compression_codec** (String) The codec used for compression of your logs. Valid values are zstd, snappy, and gzip. If the specified codec is "gzip", gzip_level will default to 3. To specify a different level, leave compression_codec blank and explicitly set the level using gzip_level. Specifying both compression_codec and gzip_level in the same API request will result in an error.
- **format** (String) Apache style log formatting.
- **format_version** (Number) The version of the custom logging format used for the configured endpoint. Can be either `1` or `2`. (default: `2`).
- **gzip_level** (Number) What level of Gzip encoding to have when dumping logs (default `0`, no compression)
- **message_type** (String) How the message should be formatted; one of: `classic`, `loggly`, `logplex` or `blank`. Default `classic`. [Fastly Documentation](https://developer.fastly.com/reference/api/logging/gcs/)
- **path** (String) Path to store the files. Must end with a trailing slash. If this field is left empty, the files will be saved in the bucket's root path
- **period** (Number) How frequently the logs should be transferred, in seconds. Default `3600`
- **placement** (String) Where in the generated VCL the logging call should be placed. Can be `none` or `waf_debug`.
- **public_key** (String) A PGP public key that Fastly will use to encrypt your log files before writing them to disk
- **response_condition** (String) The name of an existing condition in the configured endpoint, or leave blank to always execute.
- **timestamp_format** (String) specified timestamp formatting (default `%Y-%m-%dT%H:%M:%S.000`)


<a id="nestedblock--logging_scalyr"></a>
### Nested Schema for `logging_scalyr`

Required:

- **name** (String) The unique name of the Scalyr logging endpoint. It is important to note that changing this attribute will delete and recreate the resource
- **token** (String, Sensitive) The token to use for authentication (https://www.scalyr.com/keys)

Optional:

- **format** (String) Apache style log formatting.
- **format_version** (Number) The version of the custom logging format used for the configured endpoint. Can be either 1 or 2. (default: 2).
- **placement** (String) Where in the generated VCL the logging call should be placed.
- **region** (String) The region that log data will be sent to. One of `US` or `EU`. Defaults to `US` if undefined
- **response_condition** (String) The name of an existing condition in the configured endpoint, or leave blank to always execute.


<a id="nestedblock--logging_sftp"></a>
### Nested Schema for `logging_sftp`

Required:

- **address** (String) The SFTP address to stream logs to
- **name** (String) The unique name of the SFTP logging endpoint. It is important to note that changing this attribute will delete and recreate the resource
- **path** (String) The path to upload log files to. If the path ends in `/` then it is treated as a directory
- **ssh_known_hosts** (String) A list of host keys for all hosts we can connect to over SFTP
- **user** (String) The username for the server

Optional:

- **compression_codec** (String) The codec used for compression of your logs. Valid values are zstd, snappy, and gzip. If the specified codec is "gzip", gzip_level will default to 3. To specify a different level, leave compression_codec blank and explicitly set the level using gzip_level. Specifying both compression_codec and gzip_level in the same API request will result in an error.
- **format** (String) Apache-style string or VCL variables to use for log formatting.
- **format_version** (Number) The version of the custom logging format used for the configured endpoint. Can be either 1 or 2. (default: 2).
- **gzip_level** (Number) What level of Gzip encoding to have when dumping logs (default `0`, no compression)
- **message_type** (String) How the message should be formatted. One of: `classic` (default), `loggly`, `logplex` or `blank`
- **password** (String, Sensitive) The password for the server. If both `password` and `secret_key` are passed, `secret_key` will be preferred
- **period** (Number) How frequently log files are finalized so they can be available for reading (in seconds, default `3600`)
- **placement** (String) Where in the generated VCL the logging call should be placed.
- **port** (Number) The port the SFTP service listens on. (Default: `22`)
- **public_key** (String) A PGP public key that Fastly will use to encrypt your log files before writing them to disk
- **response_condition** (String) The name of the condition to apply.
- **secret_key** (String, Sensitive) The SSH private key for the server. If both `password` and `secret_key` are passed, `secret_key` will be preferred
- **timestamp_format** (String) The `strftime` specified timestamp formatting (default `%Y-%m-%dT%H:%M:%S.000`)


<a id="nestedblock--papertrail"></a>
### Nested Schema for `papertrail`

Required:

- **address** (String) The address of the Papertrail endpoint
- **name** (String) A unique name to identify this Papertrail endpoint. It is important to note that changing this attribute will delete and recreate the resource
- **port** (Number) The port associated with the address where the Papertrail endpoint can be accessed

Optional:

- **format** (String) A Fastly [log format string](https://docs.fastly.com/en/guides/custom-log-formats)
- **format_version** (Number) The version of the custom logging format used for the configured endpoint. The logging call gets placed by default in `vcl_log` if `format_version` is set to `2` and in `vcl_deliver` if `format_version` is set to `1`
- **placement** (String) Where in the generated VCL the logging call should be placed. If not set, endpoints with `format_version` of 2 are placed in `vcl_log` and those with `format_version` of 1 are placed in `vcl_deliver`
- **response_condition** (String) The name of an existing condition in the configured endpoint, or leave blank to always execute


<a id="nestedblock--request_setting"></a>
### Nested Schema for `request_setting`

Required:

- **name** (String) Unique name to refer to this Request Setting. It is important to note that changing this attribute will delete and recreate the resource

Optional:

- **action** (String) Allows you to terminate request handling and immediately perform an action. When set it can be `lookup` or `pass` (Ignore the cache completely)
- **bypass_busy_wait** (Boolean) Disable collapsed forwarding, so you don't wait for other objects to origin
- **default_host** (String) Sets the host header
- **force_miss** (Boolean) Force a cache miss for the request. If specified, can be `true` or `false`
- **force_ssl** (Boolean) Forces the request to use SSL (Redirects a non-SSL request to SSL)
- **geo_headers** (Boolean, Deprecated) Injects Fastly-Geo-Country, Fastly-Geo-City, and Fastly-Geo-Region into the request headers
- **hash_keys** (String) Comma separated list of varnish request object fields that should be in the hash key
- **max_stale_age** (Number) How old an object is allowed to be to serve `stale-if-error` or `stale-while-revalidate`, in seconds
- **request_condition** (String) Name of already defined `condition` to determine if this request setting should be applied
- **timer_support** (Boolean) Injects the X-Timer info into the request for viewing origin fetch durations
- **xff** (String) X-Forwarded-For, should be `clear`, `leave`, `append`, `append_all`, or `overwrite`. Default `append`


<a id="nestedblock--response_object"></a>
### Nested Schema for `response_object`

Required:

- **name** (String) A unique name to identify this Response Object. It is important to note that changing this attribute will delete and recreate the resource

Optional:

- **cache_condition** (String) Name of already defined `condition` to check after we have retrieved an object. If the condition passes then deliver this Request Object instead. This `condition` must be of type `CACHE`. For detailed information about Conditionals, see [Fastly's Documentation on Conditionals](https://docs.fastly.com/en/guides/using-conditions)
- **content** (String) The content to deliver for the response object
- **content_type** (String) The MIME type of the content
- **request_condition** (String) Name of already defined `condition` to be checked during the request phase. If the condition passes then this object will be delivered. This `condition` must be of type `REQUEST`
- **response** (String) The HTTP Response. Default `OK`
- **status** (Number) The HTTP Status Code. Default `200`


<a id="nestedblock--s3logging"></a>
### Nested Schema for `s3logging`

Required:

- **bucket_name** (String) The name of the bucket in which to store the logs
- **name** (String) The unique name of the S3 logging endpoint. It is important to note that changing this attribute will delete and recreate the resource

Optional:

- **acl** (String) The AWS [Canned ACL](https://docs.aws.amazon.com/AmazonS3/latest/userguide/acl-overview.html#canned-acl) to use for objects uploaded to the S3 bucket. Options are: `private`, `public-read`, `public-read-write`, `aws-exec-read`, `authenticated-read`, `bucket-owner-read`, `bucket-owner-full-control`
- **compression_codec** (String) The codec used for compression of your logs. Valid values are zstd, snappy, and gzip. If the specified codec is "gzip", gzip_level will default to 3. To specify a different level, leave compression_codec blank and explicitly set the level using gzip_level. Specifying both compression_codec and gzip_level in the same API request will result in an error.
- **domain** (String) If you created the S3 bucket outside of `us-east-1`, then specify the corresponding bucket endpoint. Example: `s3-us-west-2.amazonaws.com`
- **format** (String) Apache-style string or VCL variables to use for log formatting.
- **format_version** (Number) The version of the custom logging format used for the configured endpoint. Can be either 1 or 2. (Default: 1).
- **gzip_level** (Number) Level of Gzip compression, from `0-9`. `0` is no compression. `1` is fastest and least compressed, `9` is slowest and most compressed. Default `0`
- **message_type** (String) How the message should be formatted; one of: `classic`, `loggly`, `logplex` or `blank`. Default `classic`
- **path** (String) Path to store the files. Must end with a trailing slash. If this field is left empty, the files will be saved in the bucket's root path
- **period** (Number) How frequently the logs should be transferred, in seconds. Default `3600`
- **placement** (String) Where in the generated VCL the logging call should be placed.
- **public_key** (String) A PGP public key that Fastly will use to encrypt your log files before writing them to disk
- **redundancy** (String) The S3 storage class (redundancy level). Should be one of: `standard`, `reduced_redundancy`, `standard_ia`, or `onezone_ia`
- **response_condition** (String) Name of blockAttributes condition to apply this logging.
- **s3_access_key** (String, Sensitive) AWS Access Key of an account with the required permissions to post logs. It is **strongly** recommended you create a separate IAM user with permissions to only operate on this Bucket. This key will be not be encrypted. Not required if `iam_role` is provided. You can provide this key via an environment variable, `FASTLY_S3_ACCESS_KEY`
- **s3_iam_role** (String) The Amazon Resource Name (ARN) for the IAM role granting Fastly access to S3. Not required if `access_key` and `secret_key` are provided. You can provide this value via an environment variable, `FASTLY_S3_IAM_ROLE`
- **s3_secret_key** (String, Sensitive) AWS Secret Key of an account with the required permissions to post logs. It is **strongly** recommended you create a separate IAM user with permissions to only operate on this Bucket. This secret will be not be encrypted. Not required if `iam_role` is provided. You can provide this secret via an environment variable, `FASTLY_S3_SECRET_KEY`
- **server_side_encryption** (String) Specify what type of server side encryption should be used. Can be either `AES256` or `aws:kms`
- **server_side_encryption_kms_key_id** (String) Optional server-side KMS Key Id. Must be set if server_side_encryption is set to `aws:kms`
- **timestamp_format** (String) `strftime` specified timestamp formatting (default `%Y-%m-%dT%H:%M:%S.000`)


<a id="nestedblock--snippet"></a>
### Nested Schema for `snippet`

Required:

- **content** (String) The VCL code that specifies exactly what the snippet does
- **name** (String) A name that is unique across "regular" and "dynamic" VCL Snippet configuration blocks. It is important to note that changing this attribute will delete and recreate the resource
- **type** (String) The location in generated VCL where the snippet should be placed (can be one of `init`, `recv`, `hit`, `miss`, `pass`, `fetch`, `error`, `deliver`, `log` or `none`)

Optional:

- **priority** (Number) Priority determines the ordering for multiple snippets. Lower numbers execute first. Defaults to `100`


<a id="nestedblock--splunk"></a>
### Nested Schema for `splunk`

Required:

- **name** (String) A unique name to identify the Splunk endpoint. It is important to note that changing this attribute will delete and recreate the resource
- **url** (String) The Splunk URL to stream logs to

Optional:

- **format** (String) Apache-style string or VCL variables to use for log formatting (default: `%h %l %u %t "%r" %>s %b`)
- **format_version** (Number) The version of the custom logging format used for the configured endpoint. Can be either 1 or 2. (default: 2)
- **placement** (String) Where in the generated VCL the logging call should be placed
- **response_condition** (String) The name of the condition to apply
- **tls_ca_cert** (String) A secure certificate to authenticate the server with. Must be in PEM format. You can provide this certificate via an environment variable, `FASTLY_SPLUNK_CA_CERT`
- **tls_client_cert** (String) The client certificate used to make authenticated requests. Must be in PEM format.
- **tls_client_key** (String, Sensitive) The client private key used to make authenticated requests. Must be in PEM format.
- **tls_hostname** (String) The hostname used to verify the server's certificate. It can either be the Common Name or a Subject Alternative Name (SAN)
- **token** (String, Sensitive) The Splunk token to be used for authentication


<a id="nestedblock--sumologic"></a>
### Nested Schema for `sumologic`

Required:

- **name** (String) A unique name to identify this Sumologic endpoint. It is important to note that changing this attribute will delete and recreate the resource
- **url** (String) The URL to Sumologic collector endpoint

Optional:

- **format** (String) Apache-style string or VCL variables to use for log formatting
- **format_version** (Number) The version of the custom logging format used for the configured endpoint. Can be either 1 or 2. (Default: 1)
- **message_type** (String) How the message should be formatted; one of: `classic`, `loggly`, `logplex` or `blank`. Default `classic`. See [Fastly's Documentation on Sumologic](https://developer.fastly.com/reference/api/logging/sumologic/)
- **placement** (String) Where in the generated VCL the logging call should be placed.
- **response_condition** (String) Name of blockAttributes condition to apply this logging.


<a id="nestedblock--syslog"></a>
### Nested Schema for `syslog`

Required:

- **address** (String) A hostname or IPv4 address of the Syslog endpoint
- **name** (String) A unique name to identify this Syslog endpoint. It is important to note that changing this attribute will delete and recreate the resource

Optional:

- **format** (String) Apache-style string or VCL variables to use for log formatting
- **format_version** (Number) The version of the custom logging format. Can be either 1 or 2. (Default: 1)
- **message_type** (String) How the message should be formatted; one of: `classic`, `loggly`, `logplex` or `blank`. Default `classic`
- **placement** (String) Where in the generated VCL the logging call should be placed.
- **port** (Number) The port associated with the address where the Syslog endpoint can be accessed. Default `514`
- **response_condition** (String) Name of blockAttributes condition to apply this logging.
- **tls_ca_cert** (String) A secure certificate to authenticate the server with. Must be in PEM format. You can provide this certificate via an environment variable, `FASTLY_SYSLOG_CA_CERT`
- **tls_client_cert** (String) The client certificate used to make authenticated requests. Must be in PEM format. You can provide this certificate via an environment variable, `FASTLY_SYSLOG_CLIENT_CERT`
- **tls_client_key** (String, Sensitive) The client private key used to make authenticated requests. Must be in PEM format. You can provide this key via an environment variable, `FASTLY_SYSLOG_CLIENT_KEY`
- **tls_hostname** (String) Used during the TLS handshake to validate the certificate
- **token** (String) Whether to prepend each message with a specific token
- **use_tls** (Boolean) Whether to use TLS for secure logging. Default `false`


<a id="nestedblock--vcl"></a>
### Nested Schema for `vcl`

Required:

- **content** (String) The custom VCL code to upload
- **name** (String) A unique name for this configuration block. It is important to note that changing this attribute will delete and recreate the resource

Optional:

- **main** (Boolean) If `true`, use this block as the main configuration. If `false`, use this block as an includable library. Only a single VCL block can be marked as the main block. Default is `false`


<a id="nestedblock--waf"></a>
### Nested Schema for `waf`

Required:

- **response_object** (String) The name of the response object used by the Web Application Firewall

Optional:

- **disabled** (Boolean) A flag used to completely disable a Web Application Firewall. This is intended to only be used in an emergency
- **prefetch_condition** (String) The `condition` to determine which requests will be run past your Fastly WAF. This `condition` must be of type `PREFETCH`. For detailed information about Conditionals, see [Fastly's Documentation on Conditionals](https://docs.fastly.com/en/guides/using-conditions)

Read-Only:

- **waf_id** (String) The ID of the WAF
