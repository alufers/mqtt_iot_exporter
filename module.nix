{ self, lib, pkgs, config, ... }:

with lib;

let
  cfg = config.services.mqtt-iot-exporter;
in {
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

  config = mkIf cfg.enable {
    systemd.services.mqtt-iot-exporter = {
      description = "MQTT IoT Exporter";
      wantedBy = [ "multi-user.target" ];
      after = [ "network.target" ];

      serviceConfig = {
        ExecStart = "${self.packages.${pkgs.system}.default}/bin/mqtt-iot-exporter";
        Restart = "always";
        User = "mqtt-iot-exporter";
        Group = "mqtt-iot-exporter";
        Environment = [
          "METRICS_ADDR=${cfg.metricsAddr}"
          "MQTT_ADDR=${cfg.mqttAddr}"
          "SERVER_CERT_FILE=${toString cfg.serverCert}"
          "SERVER_KEY_FILE=${toString cfg.serverKey}"
          "CLIENT_CA_CERT=${toString cfg.clientCACert}"
          "CLIENT_CA_KEY=${toString cfg.clientCAKey}"
          "AUTOGENERATE_CLIENT_CA=${toString cfg.autogenerateClientCA}"
          "ENABLE_CLIENT_KEY_GENERATION=${toString cfg.enableClientKeyGeneration}"
        ];
      };
    };
  };
}
