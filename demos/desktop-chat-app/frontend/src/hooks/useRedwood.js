import { useMemo } from 'react'
import { usePreviousDistinct } from 'react-use'
import deepCompare from 'fast-deep-equal/es6/react'
import { useRedwood as useRedwoodJSLib } from 'redwood-client-test/react'

// *GUIDELINE: If you ever need to add additional functionality to a 3rd party hook
// Simply initialize the hook and invoke any need logic before returning the new context
function useRedwood() {
    const redwood = useRedwoodJSLib()

    const redwoodContext = useMemo(() => redwood, [redwood])

    // const prevContext = usePreviousDistinct(
    //     redwoodContext,
    //     (prev, next) => !deepCompare(prev, next),
    // )

    // console.log(prevContext, redwoodContext)
    return redwoodContext

    // Below is an example of how to extend context
    // const securityContext = useContext(securityContext)
    // return useMemo(() => Object.assign(redwoodContext, securityContext), [redwoodContext, securityContext])
}

export default useRedwood
