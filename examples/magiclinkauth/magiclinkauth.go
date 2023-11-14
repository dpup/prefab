package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/dpup/prefab/auth"
	"github.com/dpup/prefab/auth/magiclink"
	"github.com/dpup/prefab/email"
	"github.com/dpup/prefab/server"
	"github.com/dpup/prefab/templates"
	"github.com/spf13/viper"
)

func main() {
	// TODO: Consider centralizing this, maybe in server.
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error config file: %w", err))
	}

	// Initialize the server with the auth, email, and magiclink plugins, this
	// should be enough to request a magic link and authenticate a client as that
	// email account. There is no application logic or persistance.
	s := server.New(
		server.WithPlugin(auth.Plugin()),
		server.WithPlugin(email.Plugin()),
		server.WithPlugin(templates.Plugin()),
		server.WithPlugin(magiclink.Plugin()),

		server.WithHTTPHandlerFunc("/", homepage),
	)

	// Guidance for people who don't read the example code.
	fmt.Println("")
	fmt.Println("Request a magic link using:")
	fmt.Println(`curl -X POST -d '{"provider":"magiclink", "creds":{"email": "me@me.com"}}' 'http://0.0.0.0:8000/v1/auth/login'`)
	fmt.Println("")

	// Start the server.
	if err := s.Start(); err != nil {
		fmt.Println(err)
	}
}

func homepage(res http.ResponseWriter, req *http.Request) {
	res.Write([]byte(`
		<!DOCTYPE html>
		<html lang="en">
		<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<title>Prefab MagicLink Example</title>
		<script src="https://cdn.tailwindcss.com?plugins=forms"></script>
		<style>
			#confirmation, #error, #identity, #form {
				display: none;
			}
		</style>
		</head>
		<body class="flex items-center justify-center h-screen bg-gray-100">
		<div class="bg-white p-6 rounded-lg shadow-lg">
			<h1 class="text-2xl font-extrabold">Prefab MagicLink Example</h1>
			<p class="my-4 text-lg text-gray-500">This is a simple test server for demoing how magiclink auth works.</p>
			<div id="form">
				<p class="my-4 text-sm text-gray-500">Enter your email address below:</p>
				<label for="email" class="sr-only">Enter your email</label>
				<div class="flex rounded-lg shadow-sm">
					<input type="email" id="email" name="email" placeholder="me@me.com" class="py-3 px-4 block w-full border-gray-200 shadow-sm rounded-s-lg text-sm focus:z-10 focus:border-blue-500 focus:ring-blue-500 disabled:opacity-50 disabled:pointer-events-none">
					<button onclick="requestMagicLink()" type="button" class="py-3 px-4 inline-flex justify-center items-center gap-x-2 text-sm font-semibold rounded-e-md border border-transparent bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-50 disabled:pointer-events-none">
						Send
					</button>
				</div>
			</div>
			<div id="identity" class="bg-blue-100 border-t border-b border-blue-500 text-blue-700 px-4 py-3" role="alert">
				<p class="font-bold">Welcome back</p>
				<p class="text-sm" id="identity_data"></p>
			</div>
			<div id="confirmation" class="bg-blue-100 border-t border-b border-blue-500 text-blue-700 px-4 py-3" role="alert">
				<p class="font-bold">Check your inbox</p>
				<p class="text-sm">You should soon receive an email with a link you can use to authenticate.</p>
			</div>
			<div id="error" class="bg-red-100 border-t border-b border-red-500 text-red-700 px-4 py-3" role="alert">
				<p class="font-bold">An error occurred!</p>
				<p class="text-sm">This is only a demo, so check the console and the server logs.</p>
			</div>
		</div>
		<script>
		const form = 0, sent = 1, error = 2, identity = 3;
		function requestMagicLink() {
			const email = document.getElementById('email').value;
			fetch('/v1/auth/login', {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json'
				},
				credentials: 'include',
				body: JSON.stringify({
					provider: 'magiclink',
					redirectUri: window.location.href,
					creds: { email }
				})
			})
			.then(response => setMode(response.ok ? sent : error))
			.catch(error => {
				console.log('Error', error)
				setMode(error)
			});
		}
		function requestLogin(token) {
			console.log('Token detected, logging in')
			fetch('/v1/auth/login', {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json'
				},
				credentials: 'include',
				body: JSON.stringify({
					provider: 'magiclink',
					creds: { token }
				})
			})
			.then(response => {
				if (response.ok) return requestAuthUser();
				console.log('Error response', response);
				setMode(error);
				location.replace(location.pathname);
			})
			.catch(error => {
				console.log('Error', error)
				setMode(error)
			});
		}
		function requestAuthUser() {
			fetch('/v1/auth/me', {
				method: 'GET',
				credentials: 'include'
			})
			.then(response => {
				if (response.ok) {
					return response.json().then(data => {
						console.log('Identity loaded', data);
						document.getElementById('identity_data').innerText = 'Logged in as ' + data.email;
						setMode(identity);
					});
				} else {
					console.log('Error response', response)
					setMode(form);
				}
			})
			.catch(error => {
				console.log('Error', error)
				setMode(error)
			});
		}
		function setMode(mode) {
			console.log('Setting display mode', mode);
			document.getElementById('form').style.display = showIf(mode === form);
			document.getElementById('identity').style.display = showIf(mode === identity);
			document.getElementById('confirmation').style.display = showIf(mode === sent);
			document.getElementById('error').style.display = showIf(mode === error);
		}
		function showIf(state) {
			return state ? 'block' : 'none';
		}

		// Resolve the magic-link, or check to see if the user is logged in.
		const urlParams = new URLSearchParams(window.location.search);
		const token = urlParams.get('token');
		if (token) requestLogin(token);
		else requestAuthUser();
		</script>
		</body>
		</html>
	`))
}
