{
  description = "Minimal configuration MQTT to Prometheus exporter for IoT devices";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  inputs.flake-utils.url = "github:numtide/flake-utils";
  inputs.gomod2nix.url = "github:nix-community/gomod2nix";
  inputs.gomod2nix.inputs.nixpkgs.follows = "nixpkgs";
  inputs.gomod2nix.inputs.flake-utils.follows = "flake-utils";

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
      gomod2nix,
    }:
    (flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        # The current default sdk for macOS fails to compile go projects, so we use a newer one for now.
        # This has no effect on other platforms.
        callPackage = pkgs.darwin.apple_sdk_11_0.callPackage or pkgs.callPackage;
      in
      {
        packages.default = callPackage ./. {
          inherit (gomod2nix.legacyPackages.${system}) buildGoApplication;
        };
        devShells.default = callPackage ./shell.nix {
          inherit (gomod2nix.legacyPackages.${system}) mkGoEnv gomod2nix;
        };

      }
    ))
    // {
      nixosModules.default =
        {
          lib,
          pkgs,
          config,
          ...
        }:

        with lib;

        {
          options.services.mqtt-iot-exporter = {
            enable = mkEnableOption "MQTT IoT Exporter";

            metricsAddr = mkOption {
              type = types.str;
              default = "127.0.0.1:9100";
              description = "Address to serve metrics on.";
            };

            mqttAddr = mkOption {
              type = types.str;
              default = ":1883";
              description = "Address to serve MQTT on.";
            };

            serverCert = mkOption {
              type = types.nullOr types.str;
              default = null;
              description = "Path to the server certificate file.";
            };

            serverKey = mkOption {
              type = types.nullOr types.str;
              default = null;
              description = "Path to the server key file.";
            };

            clientCACert = mkOption {
              type = types.nullOr types.str;
              default = null;
              description = "Path to the client CA certificate file.";
            };

            clientCAKey = mkOption {
              type = types.nullOr types.str;
              default = null;
              description = "Path to the client CA key file.";
            };

            autogenerateClientCA = mkOption {
              type = types.bool;
              default = false;
              description = "Automatically create a client CA if it does not exist.";
            };

            enableClientKeyGeneration = mkOption {
              type = types.bool;
              default = false;
              description = "Enable client key generation.";
            };
          };

          config = mkIf config.services.mqtt-iot-exporter.enable {
            systemd.services.mqtt-iot-exporter = {
              description = "MQTT IoT Exporter";
              wantedBy = [ "multi-user.target" ];
              after = [ "network.target" ];

              serviceConfig = {
                ExecStart = "${self.packages.${pkgs.system}.default}/bin/mqtt_iot_exporter";
                Restart = "always";
                User = "mqtt-iot-exporter";
                Group = "mqtt-iot-exporter";
                Environment = [
                  "METRICS_ADDR=${config.services.mqtt-iot-exporter.metricsAddr}"
                  "MQTT_ADDR=${config.services.mqtt-iot-exporter.mqttAddr}"
                  "SERVER_CERT_FILE=${toString config.services.mqtt-iot-exporter.serverCert}"
                  "SERVER_KEY_FILE=${toString config.services.mqtt-iot-exporter.serverKey}"
                  "CLIENT_CA_CERT=${toString config.services.mqtt-iot-exporter.clientCACert}"
                  "CLIENT_CA_KEY=${toString config.services.mqtt-iot-exporter.clientCAKey}"
                  "AUTOGENERATE_CLIENT_CA=${if config.services.mqtt-iot-exporter.autogenerateClientCA then "true" else "false"}"
                  "ENABLE_CLIENT_KEY_GENERATION=${if config.services.mqtt-iot-exporter.enableClientKeyGeneration then "true" else "false"}"
                ];
              };
            };
            users.users.mqtt-iot-exporter = {
              isSystemUser = true;
              group = "mqtt-iot-exporter";
            };
            users.groups.mqtt-iot-exporter = {
            
            };
          };
        };

      formatter.x86_64-linux = nixpkgs.legacyPackages.x86_64-linux.nixfmt-rfc-style;
    };
}
