"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
const react_1 = require("react");
const RedwoodProvider_1 = require("./RedwoodProvider");
function useRedwood() {
    const redwood = react_1.useContext(RedwoodProvider_1.RedwoodContext);
    return react_1.useMemo(() => redwood, [redwood]);
}
exports.default = useRedwood;
