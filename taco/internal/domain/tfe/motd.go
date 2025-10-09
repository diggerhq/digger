package tfe

type MotdResponse struct {
	Msg string `json:"msg"`
}

func MotdMessage() string {
	// Use the bracketed tags like Terraform’s MOTD uses.
	// Terminals/clients that understand them can render colors/bold.
	// Keep \n (not newlines in a raw string) to match the JSON you showed.
	return "" +
		"                                          [cyan]-                                [reset]\n" +
		"                                          [cyan]-----                           -[reset]\n" +
		"                                          [cyan]---------                      --[reset]\n" +
		"                                          [cyan]---------  -                -----[reset]\n" +
		"                                           [cyan]---------  ------        -------[reset]\n" +
		"                                             [cyan]-------  ---------  ----------[reset]\n" +
		"                                                [cyan]----  ---------- ----------[reset]\n" +
		"                                                  [cyan]--  ---------- ----------[reset]\n" +
		"   [bold]Welcome to OpenTaco![reset]                 [cyan]-  ---------- -------[reset]\n" +
		"                                                      [cyan]---  ----- ---[reset]\n" +
		"   [bold]Documentation:[reset] opentaco.dev/docs                [cyan]--------   -[reset]\n" +
		"                                                      [cyan]----------[reset]\n" +
		"                                                      [cyan]----------[reset]\n" +
		"                                                       [cyan]---------[reset]\n" +
		"                                                           [cyan]-----[reset]\n" +
		"                                                               [cyan]-[reset]\n" +
		"\n\n" +
		"   [bold]New to OpenTaco?[reset] Try these steps to spin up quickly:\n" +
		"\n" +
		"   $ Visit digger.dev/opentaco\n" +
		"   [bold]Need help?[reset] Join: https://bit.ly/diggercommunity\n"
}
