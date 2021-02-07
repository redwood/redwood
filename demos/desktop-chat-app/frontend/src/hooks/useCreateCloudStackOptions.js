import { useEffect, useState, useMemo } from 'react'
import useAPI from './useAPI'

function useCreateCloudStackOptions(provider, apiKey) {
    const [options, setOptions] = useState({
        sshKeys: [],
        regions: [],
        instanceTypes: [],
        instanceTypesMap: [],
        images: [],
    })
    const api = useAPI()

    useEffect(() => {
        if (!apiKey || !provider || !api || !api.createCloudStackOptions) { return }
        (async function() {
            let options = await api.createCloudStackOptions(provider, apiKey)
            options.instanceTypesMap = options.instanceTypes.reduce((map, x) => ({ ...map, [x.id]: x }), {})
            setOptions(options)
        })()
    }, [api, provider, apiKey])

    return options
}

export default useCreateCloudStackOptions
