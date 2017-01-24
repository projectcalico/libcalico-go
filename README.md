[![Build Status](https://semaphoreci.com/api/v1/calico/libcalico-go/branches/master/badge.svg)](https://semaphoreci.com/calico/libcalico-go)[![Slack Status](https://slack.projectcalico.org/badge.svg)](https://slack.projectcalico.org) [![IRC Channel](https://img.shields.io/badge/irc-%23calico-blue.svg)](https://kiwiirc.com/client/irc.freenode.net/#calico)

# libcalico-go
This repositiory contains Calico's shared Go library, `libcalico`, which can be imported as 
`"github.com/projectcalico/libcalico-go/lib"`

This library is used by `calicoctl`, `calico-cni` and `felix`.

If you are looking for the golang version of `calicoctl` it has been moved to 
[its own repo](https://github.com/projectcalico/calicoctl/).
 
If you wish to use libcalico for integration with Calico networking and
policy, the main entry point to managing Calico configuration is through
the client.

-  Documentation for the client is in `lib/client`.
-  The resource structure definitions are defined in `lib/api`, this
   includes detailed per-resource and per-field level descriptions.


If you are developing against the code in libcalico, please run the tests before
submitting a Pull Request.

To run the tests locally (requires a full golang environment Go 1.7+):

    make ut
    
To run the tests within a containerized environment:

    make test-containerized
    
