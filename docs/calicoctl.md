> ![warning](images/warning.png) This document describes an alpha release of calicoctl
>
> The version of calico described in this document is an alpha release of
> calicoctl that provides management of resources using YAML and JSON
> file-based input.  The same set of commands are used for managing all 
> different resource types.
>
> This version of calicoctl does not yet contain any container-specific
> processing, this includes:
>
> -  Starting/stopping Calico node and the libnetwork plugin
> -  Management of Docker network profiles
> -  Container management commands to add/remove Calico networking from
>    and existing container.
>
> If you require any of those features, please use the latest version of
> calicoctl attached to the releases at [calico-containers](https://github.com/projectcalico/calico-containers/releases).
>
> If you are using Calico as a CNI driver, this version of calicoctl will
> allow you to manage all required Calico features, however you will need
> to start the Calico node image directly as a container.  For example,
> see [Running Calico node containers as services](https://github.com/projectcalico/calico-containers/blob/master/docs/CalicoAsService.md) for details on
> running these images as systemd services.

# calicoctl command line interface user reference

The command line tool, `calicoctl`, makes it easy to manage Calico network
and security policy.

This user reference is organized in sections based on the top level command options
of calicoctl.

## Top level help

Run `calicoctl --help` to display the following help menu for the top level 
calicoctl commands.

```
Usage: calicoctl <command> [<args>...]

    create         Create a resource by filename or stdin.
    replace        Replace a resource by filename or stdin.
    apply          Apply a resource by filename or stdin.  This creates a resource if
                   it does not exist, and replaces a resource if it does exists.
    delete         Delete a resource identified by file, stdin or resource type and name.
    get            Get a resource identified by file, stdin or resource type and name.
    version        Display the version of calicoctl.

See 'calicoctl <command> --help' to read about a specific subcommand.
```

## Top level command line options

Details on the `calicoctl` commands are described in the documents linked below
organized by top level command.

-  [calicoctl create](calicoctl/create.md)
-  [calicoctl replace](calicoctl/replace.md)
-  [calicoctl apply](calicoctl/apply.md)
-  [calicoctl delete](calicoctl/delete.md)
-  [calicoctl get](calicoctl/get.md)
-  [calicoctl version](calicoctl/version.md)

[![Analytics](https://calico-ga-beacon.appspot.com/UA-52125893-3/calico-containers/docs/calicoctl.md?pixel)](https://github.com/igrigorik/ga-beacon)
