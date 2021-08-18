---
layout: "fastly"
page_title: "Fastly: service_acl_entries_v1"
sidebar_current: "docs-fastly-resource-service-acl-entries-v1"
description: |-
  Defines a set of Fastly ACL entries that can be used to populate a service ACL. 
---

# fastly_service_acl_entries_v1

Defines a set of Fastly ACL entries that can be used to populate a service ACL.  This resource will populate an ACL with the entries and will track their state.

~> **Warning:** Terraform will take precedence over any changes you make in the UI or API. Such changes are likely to be reversed if you run Terraform again.  

If Terraform is being used to populate the initial content of an ACL which you intend to manage via API or UI, then the lifecycle `ignore_changes` field can be used with the resource.  An example of this configuration is provided below.    


## Example Usage (Terraform >= 0.12.6)

Basic usage:

```hcl
variable "myacl_name" {
	type = string
	default = "My ACL"
}

resource "fastly_service_v1" "myservice" {
  name = "demofastly"

  domain {
      name = "demo.notexample.com"
      comment = "demo"
  }

  backend {
      address = "demo.notexample.com.s3-website-us-west-2.amazonaws.com"
      name = "AWS S3 hosting"
      port = 80
    }

  acl {
	name = var.myacl_name
  }

  force_destroy = true
}

resource "fastly_service_acl_entries_v1" "entries" {
  for_each = {
    for d in fastly_service_v1.myservice.acl : d.name => d if d.name == var.myacl_name
  }
  service_id = fastly_service_v1.myservice.id
  acl_id = each.value.acl_id
  entry {
    ip = "127.0.0.1"
    subnet = "24"
    negated = false
    comment = "ALC Entry 1"
  }
}
```

Complex object usage:

The following example demonstrates the use of dynamic nested blocks to create ACL entries.

```hcl
locals {
  acl_name = "my_acl"
  acl_entries = [
    {
      ip      = "1.2.3.4"
      comment = "acl_entry_1"
    },
    {
      ip      = "1.2.3.5"
      comment = "acl_entry_2"
    },
    {
      ip      = "1.2.3.6"
      comment = "acl_entry_3"
    }
  ]
}

resource "fastly_service_v1" "myservice" {
  name = "demofastly"

  domain {
    name    = "demo.notexample.com"
    comment = "demo"
  }

  backend {
    address = "1.2.3.4"
    name    = "localhost"
    port    = 80
  }

  acl {
    name = local.acl_name
  }

  force_destroy = true
}

resource "fastly_service_acl_entries_v1" "entries" {
  for_each = {
    for d in fastly_service_v1.myservice.acl : d.name => d if d.name == local.acl_name
  }
  service_id = fastly_service_v1.myservice.id
  acl_id = each.value.acl_id
  dynamic "entry" {
    for_each = [for e in local.acl_entries : {
      ip      = e.ip
      comment = e.comment
    }]

    content {
      ip      = entry.value.ip
      subnet  = 22
      comment = entry.value.comment
      negated = false
    }
  }
}
```

## Example Usage (Terraform >= 0.12.0 && &lt; 0.12.6)

`for_each` attributes were not available in Terraform before 0.12.6, however, users can still use `for` expressions to achieve
similar behaviour as seen in the example below.

~> **Warning:** Terraform might not properly calculate implicit dependencies on computed attributes when using `for` expressions

For scenarios such as adding an ACL to a service and at the same time, creating the ACL entries (`fastly_service_acl_entries_v1`)
resource, Terraform will not calculate implicit dependencies correctly on `for` expressions. This will result in index lookup
problems and the execution will fail.

For those scenarios, it's recommended to split the changes into two distinct steps:

1. Add the `acl` block to the `fastly_service_v1` and apply the changes
2. Add the `fastly_service_acl_entries_v1` resource with the `for` expressions to the HCL and apply the changes

Usage:

```hcl
variable "myacl_name" {
	type = string
	default = "My ACL"
}

resource "fastly_service_v1" "myservice" {
  ...
  acl {
	name = var.myacl_name
  }
  ...
}

resource "fastly_service_acl_entries_v1" "entries" {
  service_id = fastly_service_v1.myservice.id
  acl_id = {for d in fastly_service_v1.myservice.acl : d.name => d.acl_id}[var.myacl_name]
  entry {
    ip = "127.0.0.1"
    subnet = "24"
    negated = false
    comment = "ALC Entry 1"
  }
}
```

### Supporting API and UI ACL updates with ignore_changes

The following example demonstrates how the lifecycle `ignore_changes` field can be used to suppress updates against the 
entries in an ACL.  If, after your first deploy, the Fastly API or UI is to be used to manage entries in an ACL, then this will stop Terraform realigning the remote state with the initial set of ACL entries defined in your HCL.

```hcl
...

resource "fastly_service_acl_entries_v1" "entries" {
  for_each = {
    for d in fastly_service_v1.myservice.acl : d.name => d if d.name == var.myacl_name
  }
  service_id = fastly_service_v1.myservice.id
  acl_id = each.value.acl_id
  entry {
    ip = "127.0.0.1"
    subnet = "24"
    negated = false
    comment = "ALC Entry 1"
  }
  
  lifecycle {
    ignore_changes = [entry,]
  }
  
}
```

## Attributes Reference

* [fastly-acl](https://developer.fastly.com/reference/api/acls/acl/)
* [fastly-acl_entry](https://developer.fastly.com/reference/api/acls/acl-entry/)

## Import

This is an example of the import command being applied to the resource named `fastly_service_acl_entries_v1.entries`
The resource ID is a combined value of the `service_id` and `acl_id` separated by a forward slash.

```
$ terraform import fastly_service_acl_entries_v1.entries xxxxxxxxxxxxxxxxxxxx/xxxxxxxxxxxxxxxxxxxx
```

If Terraform is already managing remote acl entries against a resource being imported then the user will be asked to remove it from the existing Terraform state.  
The following is an example of the Terraform state command to remove the resource named `fastly_service_acl_entries_v1.entries` from the Terraform state file.

```
$ terraform state rm fastly_service_acl_entries_v1.entries
``` 

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- **acl_id** (String) The ID of the ACL that the items belong to
- **service_id** (String) The ID of the Service that the ACL belongs to

### Optional

- **entry** (Block Set, Max: 10000) ACL Entries (see [below for nested schema](#nestedblock--entry))
- **id** (String) The ID of this resource.
- **manage_entries** (Boolean) Whether to reapply changes if the state of the entries drifts, i.e. if entries are managed externally

<a id="nestedblock--entry"></a>
### Nested Schema for `entry`

Required:

- **ip** (String) An IP address that is the focus for the ACL

Optional:

- **comment** (String) A personal freeform descriptive note
- **negated** (Boolean) A boolean that will negate the match if true
- **subnet** (String) An optional subnet mask applied to the IP address

Read-Only:

- **id** (String) The unique ID of the entry
