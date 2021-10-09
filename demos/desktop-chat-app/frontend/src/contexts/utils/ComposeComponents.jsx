import { memo } from 'react'

const ComposeComponents = ({
    components,
    children,
    componentProps = {},
    logComponentInfo = false,
}) => {
    const componentsInfo = []

    const composedComponents = components.reduceRight(
        (child, Component, idx) => {
            const componentName = Component ? Component.name : ''
            const props = componentProps[componentName]
            const renderPosition = idx + 1

            if (logComponentInfo) {
                /* eslint-disable */
                componentsInfo.push({
                    name: componentName,
                    props: props || 'No props passed.',
                    child: child.type.name,
                    renderPosition,
                })
                /* eslint-enable */
            }

            return props ? (
                <Component {...props}>{child}</Component>
            ) : (
                <Component>{child}</Component>
            )
        },
        children,
    )

    if (logComponentInfo) {
        console.log(componentsInfo)
    }

    return <>{composedComponents}</>
}

export default memo(ComposeComponents)
