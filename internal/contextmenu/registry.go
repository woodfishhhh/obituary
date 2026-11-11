package contextmenu

// Current allows process-level test isolation without opening the user registry.
// Production leaves it nil to use the current user's Windows registration.
var Current Registry
