# acpush

experimental implementation of the proposed push specification for ACIs

See https://github.com/appc/spec/issues/418 for details of the proposed specification.

## Usage
It takes as input an [ACI](https://github.com/appc/spec/blob/master/SPEC.md#app-container-image) file, an [ASC](https://github.com/coreos/rkt/blob/master/Documentation/signing-and-verification-guide.md) file, and an [App Container Name](https://github.com/appc/spec/blob/master/spec/types.md#ac-name-type) (i.e. `quay.io/coreos/etcd`).
Meta discovery is performed via the provided name to determine where to push the image to.

See `acpush --help` for details on accepted flags.

## Build

Building acpush requires go to be installed on the system.
With that, the project can be built with the following commands:

```
go get -d github.com/appc/acpush
go build github.com/appc/acpush
```

## Auth

acpush reads rkt's config files to determine what authentication is necessary for the push.
See [rkt's documentation](https://coreos.com/rkt/docs/latest/configuration.html) for details on the location and contents of these configs.
