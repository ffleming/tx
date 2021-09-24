#!/usr/bin/ruby

require 'httparty'

str = File.read("state.json");
j = JSON.parse(str)
puts HTTParty.post("http://localhost:8080/radio", {headers: {'Content-Type' => 'application/json'}, body: j.to_json})
