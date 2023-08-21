# go-splunk-rest

golang library to interact with splunk Rest API.

Supports basic, session key and authentication token methods of authentication.

Provides functions to deal with Search jobs.

Please open an issue if you want any specific Splunk API endpoints included.

## Install 

``` 
go get github.com/pvik/go-splunk-rest 
```

## Examples

To make a simple blocking search 

```go

import "github.com/pvik/go-splunk-rest"

// ... 

	splunkConn := splunk.Connection{
		Host: "https://abc.splunk.com:8089",
		AuthType: "authentication-token",
		AuthenticationToken: "abcdef111",

		// or
		//AuthType: "authorization-token" // will use session keys or set this to "basic" for basic auth
		//Username: "api-user",
		//Password: "secure-password"
	}
	
	recs, err := splunkConn.Search("| from my_datamodel | fields - _raw | head 100", splunk.SearchOptions{})
```

---

The API provides an easy way to automatically shrink the search time window if the API result return is limited to the `max_count` (typically defaults to 10000) 

Example To use this feature

```go

	splunkConn := splunk.Connection{
		Host: "https://abc.splunk.com:8089",
		AuthType: "authentication-token",
		AuthenticationToken: "abcdef111",

		// or
		//AuthType: "authorization-token" // will use session keys or set this to "basic" for basic auth
		//Username: "api-user",
		//Password: "secure-password"
	}
	
	searchOptions := splunk.SearchOptions{
		MaxCount: 100,
		
		UseEarliestTime: true,
		EarliestTime: time.Now().Sub(30*24*time.Hour),
		UseLatestTime: true,
		LatestTime: time.Now(),

		AllowPartition: true,
	}
	
	recs, err := splunkConn.Search("| from my_datamodel | fields - _raw | head 100", searchOptions)
```

--- 

The library provides an easy way to search in an async fashion 

```go

	splunkConn := splunk.Connection{
		Host: "https://abc.splunk.com:8089",
		AuthType: "authentication-token",
		AuthenticationToken: "abcdef111",

		// or
		//AuthType: "authorization-token" // will use session keys or set this to "basic" for basic auth
		//Username: "api-user",
		//Password: "secure-password"
	}
	
	go splunkConn.SearchAndExec("| from my_datamodel | fields - _raw | head 100",  splunk.SearchOptions{},
		func(results []map[string]interface{}) error {
			// do something with results 
			// this will be called once the search completes 
			
			// ... 
			
			return nil 
		}, 
		func(e error) {
			// handle search error 
			log.Errorf("search failed: %s", e)
		}
	)
```
