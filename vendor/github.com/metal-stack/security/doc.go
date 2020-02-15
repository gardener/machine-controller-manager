/*
Package security can be used to implement our bearer and hmac security
of HTTP requests. Agents can use the HMACAuth to create HMAC secured
HTTP requests. The serverside also uses this to pull out the user from
the request.

A Dex is used to get a user from a request who is identified by a bearer token.
We use a dex backend to load the keys from our service so we can verify the
signature of the JWT token. It is up to the client to get a correct bearer
token.
*/
package security
