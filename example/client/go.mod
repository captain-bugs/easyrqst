module github.com/captain-bugs/easyrqst-example

go 1.22

replace github.com/captain-bugs/easyrqst => ../../../easyrqst

require github.com/captain-bugs/easyrqst v1.0.0

require (
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.7 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
)
