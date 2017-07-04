Here is the corresponding nix service:
https://github.com/bottleofoil/nixpkgs-go-service-example/tree/go-service-example

How to test
git clone https://github.com/bottleofoil/nixpkgs-go-service-example.git 
cd nixpkgs-go-service-example
git checkout go-service-example

Enable service in configuration.nix
vi /etc/nixos/configuration.nix
services.go-service-example.enable = true;
services.go-service-example.dataDirectory = "/var/lib/go-service-example2";
services.go-service-example.host = localhost:8081;

Switch to new configuration
nixos-rebuild -I nixpkgs=/path/to/repo switch

Service should start

Check that it works as expected
echo "test content" >> /tmp/test-file.txt
curl -v http://localhost:8081/file1.txt --upload-file /tmp/test-file.txt
curl -v http://localhost:8081/file2.txt --upload-file /tmp/test-file.txt

curl -v http://localhost:8081/file2.txt
curl -X "DELETE" http://localhost:8081/file2.txt

Check logs
journalctl --unit go-service-example

Check file storage
ls /var/lib/go-service-example2/files/
du -sh /var/lib/go-service-example2/files/

