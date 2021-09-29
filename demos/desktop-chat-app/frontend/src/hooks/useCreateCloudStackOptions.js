import { useEffect, useState, useCallback } from 'react'
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

    const setCloudOptions = useCallback(async () => {
        const cloudOptions = await api.createCloudStackOptions(provider, apiKey)
        cloudOptions.instanceTypesMap = cloudOptions.instanceTypes.reduce(
            (map, x) => ({ ...map, [x.id]: x }),
            {},
        )
        setOptions(cloudOptions)
    }, [api, apiKey, provider])

    useEffect(() => {
        if (!apiKey || !provider || !api || !api.createCloudStackOptions) {
            return
        }
        setCloudOptions()
    }, [api, provider, apiKey, setCloudOptions])

    return options
}

export default useCreateCloudStackOptions
