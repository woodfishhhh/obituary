// Window preferences kept on this computer only, like the look.

const ASK = "obit.askDecisions";

// Whether approving or rejecting a proposal asks first. Default: it asks.
export const askDecisions = () => localStorage.getItem(ASK) !== "no";
export const setAskDecisions = (ask: boolean) => (ask ? localStorage.removeItem(ASK) : localStorage.setItem(ASK, "no"));
