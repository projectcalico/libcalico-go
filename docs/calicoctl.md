> ![warning](images/warning.png) This document describes an alpha release of calicoctl
>
> The version of calico
> View the calico-containers documentation for the latest release [here](https://github.com/projectcalico/calico-containers/blob/v0.21.0/README.md).

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

-  [calicoctl status](calicoctl/status.md)
-  [calicoctl node](calicoctl/node.md)
-  [calicoctl container](calicoctl/container.md)
-  [calicoctl profile](calicoctl/profile.md)
-  [calicoctl endpoint](calicoctl/endpoint.md)
-  [calicoctl pool](calicoctl/pool.md)
-  [calicoctl bgp](calicoctl/bgp.md)
-  [calicoctl ipam](calicoctl/ipam.md)
-  [calicoctl checksystem](calicoctl/checksystem.md)
-  [calicoctl diags](calicoctl/diags.md)
-  [calicoctl version](calicoctl/version.md)
-  [calicoctl config](calicoctl/config.md)

[![Analytics](https://calico-ga-beacon.appspot.com/UA-52125893-3/calico-containers/docs/calicoctl.md?pixel)](https://github.com/igrigorik/ga-beacon)
