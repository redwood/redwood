const withContextProvider =
    (Provider, providerProps) => (Component) => (props) =>
        (
            <Provider {...providerProps}>
                <Component {...props} />
            </Provider>
        )

export default withContextProvider
