/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { CreatedPost } from '../models/CreatedPost';
import type { ListPostsResponseModel } from '../models/ListPostsResponseModel';
import type { PostModel } from '../models/PostModel';
import type { CancelablePromise } from '../core/CancelablePromise';
import type { BaseHttpRequest } from '../core/BaseHttpRequest';
export class DefaultService {
    constructor(public readonly httpRequest: BaseHttpRequest) {}
    /**
     * Get image of the post by ID
     * @returns any Post image
     * @throws ApiError
     */
    public getPostImage({
        id,
    }: {
        id: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/posts/{id}/image',
            path: {
                'id': id,
            },
            errors: {
                400: `bad request`,
                404: `not found`,
            },
        });
    }
    /**
     * Get a list of posts
     * @returns ListPostsResponseModel A list of posts
     * @throws ApiError
     */
    public listPosts({
        page = 1,
        pageSize = 10,
        sortOrder = 'desc',
    }: {
        page?: number,
        pageSize?: number,
        sortOrder?: 'asc' | 'desc',
    }): CancelablePromise<ListPostsResponseModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/posts/',
            query: {
                'page': page,
                'page_size': pageSize,
                'sort_order': sortOrder,
            },
        });
    }
    /**
     * Create a new post
     * @returns CreatedPost Post created successfully
     * @throws ApiError
     */
    public createPost({
        formData,
    }: {
        formData: {
            title?: string;
            description?: string;
            text?: string;
            preview_image?: Blob;
        },
    }): CancelablePromise<CreatedPost> {
        return this.httpRequest.request({
            method: 'POST',
            url: '/posts/',
            formData: formData,
            mediaType: 'multipart/form-data',
            errors: {
                400: `bad request`,
                401: `unauthorized`,
                403: `forbidden`,
            },
        });
    }
    /**
     * Get a single post by ID
     * @returns PostModel A single post
     * @throws ApiError
     */
    public getPostById({
        id,
    }: {
        id: string,
    }): CancelablePromise<PostModel> {
        return this.httpRequest.request({
            method: 'GET',
            url: '/posts/{id}',
            path: {
                'id': id,
            },
            errors: {
                400: `bad request`,
                404: `not found`,
            },
        });
    }
    /**
     * Partially update a post by ID
     * @returns any Post partially updated successfully
     * @throws ApiError
     */
    public patchPostById({
        id,
        formData,
    }: {
        id: string,
        formData?: {
            title?: string;
            description?: string;
            text?: string;
            preview_image?: Blob;
        },
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'PATCH',
            url: '/posts/{id}',
            path: {
                'id': id,
            },
            formData: formData,
            mediaType: 'multipart/form-data',
            errors: {
                400: `bad request`,
                401: `unauthorized`,
                403: `forbidden`,
                404: `not found`,
            },
        });
    }
    /**
     * Delete a post by ID
     * @returns any Post deleted successfully
     * @throws ApiError
     */
    public deletePostById({
        id,
    }: {
        id: string,
    }): CancelablePromise<any> {
        return this.httpRequest.request({
            method: 'DELETE',
            url: '/posts/{id}',
            path: {
                'id': id,
            },
            errors: {
                400: `bad request`,
                401: `unauthorized`,
                403: `forbidden`,
                404: `not found`,
            },
        });
    }
}
