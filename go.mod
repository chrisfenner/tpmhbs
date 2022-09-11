module github.com/chrisfenner/tpmhbs

go 1.19

require (
	github.com/google/go-tpm v0.3.3
	github.com/jedib0t/go-pretty/v6 v6.3.8
	github.com/sajari/regression v1.0.1
	github.com/schollz/progressbar/v3 v3.11.0
)

require (
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/mitchellh/colorstring v0.0.0-20190213212951-d06e56a500db // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	golang.org/x/sys v0.0.0-20220909162455-aba9fc2a8ff2 // indirect
	golang.org/x/term v0.0.0-20220722155259-a9ba230a4035 // indirect
	gonum.org/v1/gonum v0.12.0 // indirect
)

replace github.com/google/go-tpm => github.com/chrisfenner/go-tpm v0.3.4-0.20220911015222-b47f2a08430e
