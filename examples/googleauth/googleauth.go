package main

import (
	"fmt"
	"net/http"

	"github.com/dpup/prefab/auth"
	"github.com/dpup/prefab/auth/google"
	"github.com/dpup/prefab/server"
)

func main() {
	server.LoadDefaultConfig()

	s := server.New(
		server.WithPlugin(auth.Plugin()),
		server.WithPlugin(google.Plugin()),

		server.WithHTTPHandlerFunc("/", homepage),
	)

	fmt.Println("")
	fmt.Println("Visit http://localhost:8000/ in your browser")
	fmt.Println("")

	// Start the server.
	if err := s.Start(); err != nil {
		fmt.Println(err)
	}
}

// Google auth task list:
// TODO: Update login endpoint to accept id_token and decode it.
// TODO: Add google SDK to this example, and use it to trigger a client side login flow.

func homepage(res http.ResponseWriter, req *http.Request) {
	res.Write([]byte(`
		<!DOCTYPE html>
		<html lang="en">
		<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<title>Prefab Google Example</title>
		<script src="https://cdn.tailwindcss.com?plugins=forms"></script>
		<style>

		</style>
		</head>
		<body class="flex items-center justify-center h-screen bg-gray-100">
		<div class="bg-white p-6 rounded-lg shadow-lg">
			<h1 class="text-2xl font-extrabold">Prefab Google Example</h1>
			<p class="my-4 text-lg text-gray-500">This is a simple test server for demoing how google auth works.</p>
			<div id="buttons" class="hidden">
				<p><a href="/api/auth/login?provider=google&redirect_uri=/">Login with google server side flow &raquo;</a></p>
			</div>
			<div id="identity" class="hidden bg-blue-100 border-t border-b border-blue-500 text-blue-700 px-4 py-3" role="alert">
				<p class="font-bold">Welcome back</p>
				<p class="text-sm" id="identity_data"></p>
			</div>
		</div>
		<script>
		const buttons = 0, identity = 2;
		function requestAuthUser() {
			fetch('/api/auth/me', {
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
					setMode(buttons);
				}
			})
			.catch(error => {
				console.log('Error', error)
				setMode(error)
			});
		}
		function setMode(mode) {
			console.log('Setting display mode', mode);
			document.getElementById('buttons').style.display = showIf(mode === buttons);
			document.getElementById('identity').style.display = showIf(mode === identity);
		}
		function showIf(state) {
			return state ? 'block' : 'none';
		}
		requestAuthUser();
		</script>
		</body>
		</html>
	`))
}
