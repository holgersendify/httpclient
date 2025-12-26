## reponse struct
do we really need the json, xml etc methods on the response struct? it the get, post etc methods already supports passing in body and resp types for parsing. We could most likley have a slimer api here.
or can you see a good reason for having this way? please provide examples on cases this might be useful.


## logging middliware.

Logging of the request / response should be on by default.

what logger you want to use should be setup with the client. We will default to a simple slog json log.

I want to give each client a custom third_party_code that will be used when logging. 

We should support structurd logging with this interface for the logger func
```go
func Log(ctx context.Context, level slog.Level, msg string, attrs []slog.Attr) 
```