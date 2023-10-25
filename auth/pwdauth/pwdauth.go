// Package pwdauth provides an authentication service plugin that allows users
// to authenticate via a username (or email) and password.
package pwdauth

/*
# Username / password auth Plan

(Demo version)

Needs:

- Login endpoint:
  - takes username/password
  - sets cookie or returns an auth token
  - if cookie is set, needs a redirect
    - 302 with cookie should work, but won't on localhost
    - could use a meta-redirect if dest is localhost or different domain
- Source for user credentials - needs to be pluggable
  - for sample, plain text data file.


Ideas:
- Prefab has it's own `AuthService` provides the basic mechanics for authentication.
- Authentication providers plugin to the service.
- Does the request hint at what authenticator to use, or should it be duck typed?
- Auth provider used should be encoded in the token some how
*/
