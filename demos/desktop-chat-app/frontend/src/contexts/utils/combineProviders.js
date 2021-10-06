const combineProviders = (...components) =>
    components.reduce(
        (AccumulatedComponents, CurrentComponent) =>
            ({ children }) =>
                (
                    <AccumulatedComponents>
                        <CurrentComponent>{children}</CurrentComponent>
                    </AccumulatedComponents>
                ),
        ({ children }) => <>{children}</>,
    )

export default combineProviders
