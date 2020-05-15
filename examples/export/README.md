# Sample Export File

This is a sample export file to help understand the format documented in
the [Exposure Key File Format documentation](https://www.google.com/covid19/exposurenotifications/pdfs/Exposure-Key-File-Format-and-Verification.pdf)

The contents of the archive:

```shell
$ unzip -l sample-export.zip
Archive:  sample-export.zip
  Length      Date    Time    Name
---------  ---------- -----   ----
      137  1980-00-00 00:00   export.bin
      120  1980-00-00 00:00   export.sig
---------                     -------
      257                     2 files
```

The signature can be extracted from `export.sig` and verified
with [OpenSSL](https://www.openssl.org/) as follows:

```shell
$ go run ./tools/unwrap-signature/ --in=export.sig --out=sigRaw
...
$ openssl dgst -sha256 -verify public.pem -signature sigRaw export.bin
Verified OK
```

The contents of `export.bin` includes a fixed width header in addition to
the serialized proto. It can be decoded at the commandline by skipping the
header with `tail`:

```
$ tail +17c < export.bin | protoc --decode TemporaryExposureKeyExport export.proto
start_timestamp: 1589490000
end_timestamp: 1589493600
...
```

