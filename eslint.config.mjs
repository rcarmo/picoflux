// Flat configuration for the embedded browser scripts; preserve existing rules.
export default [{
    files: ["internal/ui/static/js/*.js"],
    languageOptions: { ecmaVersion: 2020, sourceType: "script" },
    rules: { indent: ["error", 4] },
}];
