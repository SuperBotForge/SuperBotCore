import {Configuration, type FetchAPI} from "./api-client";

export const apiConf = (fetchApi: FetchAPI = fetch) =>
    new Configuration({
        fetchApi: fetchApi,
        headers: {"Authorization": "Bearer " + window.location.hash.substring(1)},
        basePath: import.meta.env.VITE_API_URL.replace(/\/$/, ""), // trim last slash
        middleware: [
            {
                post: async ({response}) => {
                    if (response.status > 399) {
                        const err = await response.json();
                        const errMsg = err.message || err.errors || err.error_message || err.error
                        alert(`${response.status} ${errMsg}`);
                    }
                }
            }
        ]
    });
