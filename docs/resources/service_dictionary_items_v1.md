---
layout: "fastly"
page_title: "Fastly: service_dictionary_items_v1"
sidebar_current: "docs-fastly-resource-service-dictionary-items-v1"
description: |-
  Provides a grouping of Fastly dictionary items that can be applied to a service. 
---

# fastly_service_dictionary_items_v1

Defines a map of Fastly dictionary items that can be used to populate a service dictionary.  This resource will populate a dictionary with the items and will track their state.


~> **Warning:** Terraform will take precedence over any changes you make in the UI or API. Such changes are likely to be reversed if you run Terraform again.  

If Terraform is being used to populate the initial content of a dictionary which you intend to manage via API or UI, then the lifecycle `ignore_changes` field can be used with the resource.  An example of this configuration is provided below.    

## Limitations

- `write_only` dictionaries are not supported

## Example Usage (Terraform >= 0.12.6)

Basic usage:

```hcl
variable "mydict_name" {
	type = string
	default = "My Dictionary"
}

resource "fastly_service_v1" "myservice" {
  name = "demofastly"

  domain {
    name    = "demo.notexample.com"
    comment = "demo"
  }

  backend {
    address = "demo.notexample.com.s3-website-us-west-2.amazonaws.com"
    name    = "AWS S3 hosting"
    port    = 80
  }

  dictionary {
	name       = var.mydict_name
  }

  force_destroy = true
}

resource "fastly_service_dictionary_items_v1" "items" {
  for_each = {
    for d in fastly_service_v1.myservice.dictionary : d.name => d if d.name == var.mydict_name
  }
  service_id = fastly_service_v1.myservice.id
  dictionary_id = each.value.dictionary_id

  items = {
    key1: "value1"
    key2: "value2"
  }
}
```

Complex object usage:

```hcl
variable "mydict" {
  type = object({ name=string, items=map(string) })
  default = {
    name = "My Dictionary"
    items = {
      key1: "value1x"
      key2: "value2x"
    }
  }
}

resource "fastly_service_v1" "myservice" {
  name = "demofastly"

  domain {
    name    = "demo.notexample.com"
    comment = "demo"
  }

  backend {
    address = "demo.notexample.com.s3-website-us-west-2.amazonaws.com"
    name    = "AWS S3 hosting"
    port    = 80
  }

  dictionary {
    name = var.mydict.name
  }

  force_destroy = true
}

resource "fastly_service_dictionary_items_v1" "items" {
  for_each = {
    for d in fastly_service_v1.myservice.dictionary : d.name => d if d.name == var.mydict.name
  }
  service_id = fastly_service_v1.myservice.id
  dictionary_id = each.value.dictionary_id
  items = var.mydict.items
}
```

Expression and functions usage:

```hcl
// Local variables used when formatting values for the "My Project Dictionary" example
locals {
  dictionary_name = "My Project Dictionary"
  host_base = "demo.ocnotexample.com"
  host_divisions = ["alpha", "beta", "gamma", "delta"]
}

// Define the standard service that will be used to manage the dictionaries.
resource "fastly_service_v1" "myservice" {
  name = "demofastly"

  domain {
    name = "demo.ocnotexample.com"
    comment = "demo"
  }

  backend {
    address = "demo.ocnotexample.com.s3-website-us-west-2.amazonaws.com"
    name = "AWS S3 hosting"
    port = 80
  }

  dictionary {
    name = local.dictionary_name
  }

  force_destroy = true
}

// This resource is dynamically creating the items from the local variables through for expressions and functions.
resource "fastly_service_dictionary_items_v1" "project" {
  for_each = {
    for d in fastly_service_v1.myservice.dictionary : d.name => d if d.name == local.dictionary_name
  }
  service_id = fastly_service_v1.myservice.id
  dictionary_id = each.value.dictionary_id
  items = {
    for division in local.host_divisions:
      division => format("%s.%s", division, local.host_base)
  }
}
```

## Example Usage (Terraform >= 0.12.0 && &lt; 0.12.6)

`for_each` attributes were not available in Terraform before 0.12.6, however, users can still use `for` expressions to achieve
similar behaviour as seen in the example below.

~> **Warning:** Terraform might not properly calculate implicit dependencies on computed attributes when using `for` expressions

For scenarios such as adding a Dictionary to a service and at the same time, creating the Dictionary entries (`fastly_service_dictionary_items_v1`)
resource, Terraform will not calculate implicit dependencies correctly on `for` expressions. This will result in index lookup
problems and the execution will fail.

For those scenarios, it's recommended to split the changes into two distinct steps:

1. Add the `dictionary` block to the `fastly_service_v1` and apply the changes
2. Add the `fastly_service_dictionary_items_v1` resource with the `for` expressions to the HCL and apply the changes

Usage:

```hcl
variable "mydict_name" {
	type = string
	default = "My Dictionary"
}

resource "fastly_service_v1" "myservice" {
  ...
  dictionary {
	name       = var.mydict_name
  }
  ...
}

resource "fastly_service_dictionary_items_v1" "items" {
  service_id = fastly_service_v1.myservice.id
  dictionary_id = {for s in fastly_service_v1.myservice.dictionary : s.name => s.dictionary_id}[var.mydict_name]

  items = {
    key1: "value1"
    key2: "value2"
  }
}
```

### Supporting API and UI dictionary updates with ignore_changes

The following example demonstrates how the lifecycle `ignore_changes` field can be used to suppress updates against the 
items in a dictionary.  If, after your first deploy, the Fastly API or UI is to be used to manage items in a dictionary, then this will stop Terraform realigning the remote state with the initial set of dictionary items defined in your HCL.

```hcl
...

resource "fastly_service_dictionary_items_v1" "items" {
  for_each = {
    for d in fastly_service_v1.myservice.dictionary : d.name => d if d.name == var.mydict_name
  }
  service_id = fastly_service_v1.myservice.id
  dictionary_id = each.value.dictionary_id

  items = {
    key1: "value1"
    key2: "value2"
  }

  lifecycle {
    ignore_changes = [items,]
  }
}
```

## Attributes Reference

* [fastly-dictionary](https://developer.fastly.com/reference/api/dictionaries/dictionary/)
* [fastly-dictionary_item](https://developer.fastly.com/reference/api/dictionaries/dictionary-item/)

## Import

This is an example of the import command being applied to the resource named `fastly_service_dictionary_items_v1.items`
The resource ID is a combined value of the `service_id` and `dictionary_id` separated by a forward slash.

```
$ terraform import fastly_service_dictionary_items_v1.items xxxxxxxxxxxxxxxxxxxx/xxxxxxxxxxxxxxxxxxxx
```

If Terraform is already managing remote dictionary items against a resource being imported then the user will be asked to remove it from the existing Terraform state.  
The following is an example of the Terraform state command to remove the resource named `fastly_service_dictionary_items_v1.items` from the Terraform state file.

```
$ terraform state rm fastly_service_dictionary_items_v1.items
``` 
<!-- schema generated by tfplugindocs -->
## Schema

### Required

- **dictionary_id** (String) The ID of the dictionary that the items belong to
- **service_id** (String) The ID of the service that the dictionary belongs to

### Optional

- **id** (String) The ID of this resource.
- **items** (Map of String) A map representing an entry in the dictionary, (key/value)
- **manage_items** (Boolean) Whether to reapply changes if the state of the items drifts, i.e. if items are managed externally
