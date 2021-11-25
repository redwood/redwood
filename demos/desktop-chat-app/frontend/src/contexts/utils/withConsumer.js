const destruct = (obj, ...keys) =>
    keys.reduce((a, c) => ({ ...a, [c]: obj[c] }), {})

function ConsumerComponent(Component, Context, propKeys) {
    return function CreatedComponent(props) {
        return (
            <Context.Consumer>
                {(consumerProps) => {
                    let selectedConsumerProps = consumerProps
                    if (propKeys) {
                        selectedConsumerProps = destruct(
                            consumerProps,
                            propKeys,
                        )
                    }

                    return <Component {...props} {...selectedConsumerProps} />
                }}
            </Context.Consumer>
        )
    }
}

const withConsumer = (Component, Context, propKeys) =>
    ConsumerComponent(Component, Context, propKeys)

//
// function ConsumerComponent(props) {
//     return (
//         <Context.Consumer>
//             {(consumerProps) => {
//                 let selectedConsumerProps = consumerProps
//                 if (propKeys) {
//                     selectedConsumerProps = destruct(
//                         consumerProps,
//                         propKeys,
//                     )
//                 }

//                 return <Component {...props} {...selectedConsumerProps} />
//             }}
//         </Context.Consumer>
//     )
// }

export default withConsumer
