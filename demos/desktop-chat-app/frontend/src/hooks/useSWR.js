import useSWRHook from 'swr'
import fetch from 'unfetch'

const fetcher = (...args) => fetch(...args).then((res) => res)

function useSWR(url, options = {}) {
    const { data, error, isValidating, mutate } = useSWRHook(
        url,
        fetcher,
        options,
    )

    return {
        data,
        error,
        isValidating,
        mutate,
    }
}

export default useSWR
