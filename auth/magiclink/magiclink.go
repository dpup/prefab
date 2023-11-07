// Package magiclink provides and authentication service pluging that allows
// users to authenticate using a magic link that is sent to their email address.
package magiclink

/*
Plan

- Login endpoint:
  - takes email
	- generates signed JWT with email and expiration
	- sends email with the magic-link
- Login endpoint:
	- takes token from magic-link
	- extracts email from token and creates auth token
	- sets cookie or returns an auth token
  - if cookie is set, needs a redirect
    - 302 with cookie should work, but won't on localhost
    - could use a meta-redirect if dest is localhost or different domain

What's needed:
- AuthService plugin for handling basic login endpoint, required by magiclink plugin
		- Login RPC
		- LoginRequest has key/value pairs (proto3 doesn't support extend) OR has a
		  details field which accepts google.protobuf.Any
- EmailSender, could just use SMTP or could be an abstract interface.
- Template for the email


Questions:
- should cookies only be set if there's a valid user?
- if so, what is the underlying source of user data?
- can it just be a pluggin?
- can this just say: this user has verified they have access to this email? And
  then ACL code should verify that the authenticated user is authorized to
  access the resource.
- can a browser have multiple authenticated identities
*/
