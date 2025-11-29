'use client';

import React, {useEffect, useRef, useState} from 'react';
import {SubmissionsList} from '@/components/SubmissionsList';
import type {ListSubmissionsResponseModel, SubmissionsListItemModel} from '../../../contracts/core/v1';

const MessageTypeCreate = "CREATE";
const MessageTypeUpdate = "UPDATE";
const MessageTypeDelete = "DELETE";

interface Message {
    message_type: string;
    message?: string;
    submission: SubmissionsListItemModel;
}

interface SubmissionsListWithWebSocketProps {
    initialSubmissions: ListSubmissionsResponseModel;
    wsUrl: string;
    token: string;
    queryParams: Record<string, string | number | undefined>;
}

const SubmissionsListWithWS: React.FC<SubmissionsListWithWebSocketProps> = (
    {
        initialSubmissions,
        wsUrl,
        token,
        queryParams
    }
) => {
    const [submissions, setSubmissions] = useState<SubmissionsListItemModel[]>(initialSubmissions.submissions);
    const ws = useRef<WebSocket | null>(null);

    useEffect(() => {
        console.log(queryParams)

        if (!(queryParams.page === 1 && queryParams.order === -1)) {
            return;
        }

        const websocket =  new WebSocket(`${wsUrl}?token=${token}`);

        websocket.onopen = () => {
            console.log('WebSocket connected');
        };

        websocket.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data) as Message;

                if (data.message_type === MessageTypeCreate) {

                    setSubmissions((prevSubmissions) => [data.submission, ...prevSubmissions.slice(0, -1)]);
                    return;
                }

                if (data.message_type === MessageTypeUpdate) {

                    setSubmissions((prevSubmissions) =>
                        prevSubmissions.map((submission) => {
                            if (submission.id === data.submission.id) {
                                if (data.message) {
                                    // FIXME: Backend sends state as string in WebSocket message,
                                    // but SubmissionsListItemModel.state is typed as number.
                                    // This needs to be fixed on the backend or model adjusted.
                                    (data.submission as SubmissionsListItemModel & { state: string | number }).state = data.message;
                                }

                                return data.submission;
                            }

                            return submission;
                        })
                    );
                    return;
                }

                if (data.message_type === MessageTypeDelete) {

                    setSubmissions((prevSubmissions) =>
                        prevSubmissions.filter((submission) => submission.id !== data.submission.id)
                    );
                    return;
                }
            } catch (error) {
                console.error('WebSocket message error:', error);
            }
        };

        websocket.onerror = (error) => {
            console.error('WebSocket error:', error);
        };

        websocket.onclose = () => {
            console.log('WebSocket disconnected');
            ws.current = null;
        };

        ws.current = websocket;

        return () => {
            if (websocket && websocket.readyState === WebSocket.OPEN) {
                websocket.close();
            }
        };
    }, [wsUrl, token, queryParams]);

    useEffect(() => {
        setSubmissions(initialSubmissions.submissions);
    }, [initialSubmissions]);

    return <SubmissionsList submissions={submissions}/>;
};

export {SubmissionsListWithWS};
