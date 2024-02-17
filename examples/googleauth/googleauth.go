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
// TODO: Handler at /api/auth/google/callback to manage authorization code.
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
			<p><a href="/api/auth/login?provider=google">Login with google server side flow &raquo;</a></p>
		</div>
		<script>

		</script>
		</body>
		</html>
	`))
}
