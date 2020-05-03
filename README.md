# Insurgency Sandstorm Receptionist

The ISR is a application for your Insurgency Sandstorm server. It checks every 5 seconds the online players and tells changes through the RCON „say“ console command.

## Features

At the moment three features are implemented:

- Mentioning new joined players by greeting them
- Mentioning players who left the server
- Telling joined players the [server rules](#rules-file) (optional)


## Run the ISR

To start the ISR make sure you have a config in the current directory (see example configuration below). There is one optional parameter: you can specify a different config file.

## Example configuration

```ini
# hostname or ip of your server
host                = my.fancy.server
# server's query port
queryPort           = 27131
# server's RCON port
rconPort            = 27015
# RCON password
rconPassword        = supersecret
# welcome message, @ is replaced with players name
templateWelcome     = Welcome, @!
# farewell message
templateFarewell    = Player @ just left.
# rules file
rulesFile           = rules.txt
# delay until first rule is printed in seconds
tellFirstRuleDelay = 30
# delay between every rule in seconds
tellNextRulesDelay = 10
```
Besides the `host` and the `rconPassword` all parameter are optional. The default values can be found in the example configuration.

## Rules file

The syntax of the rules file is quite simple: one rule per line. After a player joined the server, ISR waits `tellFirstRuleDelay` amount of seconds and starts telling the rules. The is because the player needs to equip and doesn‘t see the in-game chat. Every rule is delayed `tellNextRulesDelay` seconds.

Rule telling be disabled by simply omitting the rules file.

## Known issues

In our tests it was not possible to connect to the RCON of a Insurgency Sandstorm server running on the same machine as the ISR via the loopback device („localhost“ / `127.0.0.1`). So please use the LAN IP or the hostname in this case.

## License

ISR is licensed under the [MIT license](blob/master/LICENSE).