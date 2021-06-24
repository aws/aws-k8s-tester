```bash
#All Tests
go test -timeout=0 -v -ginkgo.v ./security_test.go

#Single Test
go test -timeout=0 -v -ginkgo.v -ginkgo.focus="security-falco*" ./security_test.go
```