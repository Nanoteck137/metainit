empty:

gen:
	curl https://raw.githubusercontent.com/Nanoteck137/watchbook/refs/heads/main/misc/pyrin.json -o dwebble-pyrin.json
	nix run github:nanoteck137/pyrin -- gen go ./dwebble-pyrin.json -o watchbook
	find ./watchbook -type f -exec sed -i '' -e 's/package api/package watchbook/g' {} \;
