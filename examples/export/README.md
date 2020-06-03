# Working with Export Files

This demonstrates some utilities and commands to help understand the format
documented in the
[Exposure Key File Format documentation](https://www.google.com/covid19/exposurenotifications/pdfs/Exposure-Key-File-Format-and-Verification.pdf)

## Generating signing keys

Export files are signed and verified in the Exposure Notification API prior to
matching. Any standard cryptography tools can be used to generate the asymmetric
key pair used to sign the files. The following illustrates how this can be done
with [OpenSSL](https://www.openssl.org/).

To generate the private key used for signing:

```shell
$ openssl ecparam -genkey -name prime256v1 -noout -out private.pem
```

`private.pem` would not be shared. For verification, the corresponding public
key would need to be shared. The public key could be generated as follows.
Again, any standard cryptography tools can be used, including key management
services. This just illustrates one way it can be done, and how the files in the
repo were created.

```shell
$ openssl ec -in private.pem -pubout -out public.pem
```

## Generating an export

There is a utility that can generate either a random export file or use the JSON
format in the publish api call to specify the contents. The export file in this
directory was generated with the keys above and the `keys.json` file as follows:

```shell
$ go run ./tools/export-generate --signing-key=./examples/export/private.pem --tek-file=./examples/export/keys.json
```

## Inspecting an export

Exports are just zip files whose contents can be examined as follows:

```shell
$ unzip -l testExport-2-records-1-of-1.zip
Archive:  testExport-2-records-1-of-1.zip
  Length      Date    Time    Name
---------  ---------- -----   ----
      135  1980-00-00 00:00   export.bin
      114  1980-00-00 00:00   export.sig
---------                     -------
      249                     2 files
```

The contents of `export.bin` includes a fixed width header in addition to the
serialized proto. It can be decoded at the commandline by skipping the header
with `tail`:

```
$ tail +17c < export.bin | protoc --decode TemporaryExposureKeyExport --proto_path ../../internal/pb/export/ export.proto | head -n 10
start_timestamp: 1588345200
end_timestamp: 1588431600
region: "US"
batch_num: 1
batch_size: 1
signature_infos {
  verification_key_version: "1"
  verification_key_id: "some_id"
  signature_algorithm: "1.2.840.10045.4.3.2"
}
...
```

## Verifying an export

There's a handy utility to extract the raw bytes of the signature from the
SignatureInfo proto in the `export.sig` file. Then you can use any standard
tools to verify the signature.

```shell
$ go run ./tools/unwrap-signature/ --in=export.sig --out=sigRaw
...
$ openssl dgst -sha256 -verify public.pem -signature sigRaw export.bin
Verified OK
```
