# cartographer-agent

Cartographer-Agent is a lightweight system inventory tool written in Go. It runs on your servers to collect various system information and reports them to a centralized server via REST api. Cartographer-Agent helps you maintain a real-time map of your infrastructure.

This project has been around for a few years, on a private repo, but it is being used by a few people so I decided to finally open source it. The server isn't open source (yet), but this will post to any REST api so it'd be easy to use the data in your systems. 

NOTE: I decided not to convert the private repo to public as I didn't feel like cleaning up the git history, so the code present is a snapshot of the current state of the project moved to a fresh repo. (actually, the repo is two years old as I intended to do this a long time ago).  