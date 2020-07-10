# lxd_project

Manages an LXD project.

## Example Usage

```hcl

resource "lxd_project" "myproject" {
  name = "greatproject"
  description = "My great project isolated from other lxd things"

  config = {
    "limits.cpu" = 1
  }
}

resource "lxd_profile" "profile1" {
  name = "profile1"
  project = lxd_project.myproject.name

  config = {
    "limits.cpu" = 2
  }

  device {
    name = "shared"
    type = "disk"

    properties = {
      source = "/tmp"
      path   = "/tmp"
    }
  }

  device {
    type = "disk"
    name = "root"

    properties = {
      pool = "default"
      path = "/"
    }
  }
}

resource "lxd_container" "test1" {
  name      = "test1"
  image     = "ubuntu:bionic"
  ephemeral = false
  profiles  = ["default", lxd_profile.profile1.name]
  project = lxd_project.myproject.name
}
```

## Argument Reference

* `remote` - *Optional* - The remote in which the resource will be created. If
	it is not provided, the default provider remote is used.

* `name` - *Required* - Name of the project.

* `config` - *Optional* - Map of key/value pairs of
	[project config settings](https://linuxcontainers.org/lxd/docs/master/projects).

* `description` - *Optional* - A readeable project description

## Importing

Profiles can be imported by doing:

```shell
$ terraform import lxd_project.my_project <name of project>
```

## Notes

* Project is compatible with and allows you isolate: profiles, images, volumes and containers