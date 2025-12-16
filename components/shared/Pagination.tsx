'use client';

import React from 'react';
import {Pagination} from '@mantine/core';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import * as corev1 from '@contracts/core/v1';
import { usePageTransition } from '@/components/workshop/WorkshopPageWrapper';

interface NextPaginationProps {
    pagination: corev1.PaginationModel;
    baseUrl: string;
    queryParams?: Record<string, string | number | undefined>;
}

const NextPagination = ({pagination, baseUrl, queryParams = {}}: NextPaginationProps) => {
    const router = useRouter();
    const transitionContext = usePageTransition();
    const [isPending, reactStartTransition] = React.useTransition();

    // Helper function to build query string
    const buildQueryString = (params: Record<string, string | number | undefined>) => {
        const validParams = Object.entries(params)
            .filter(([_, value]) => value !== undefined && value !== '')
            .map(([key, value]) => `${key}=${encodeURIComponent(value!)}`);
        return validParams.length > 0 ? `?${validParams.join('&')}` : '';
    };

    const navigateToPage = (page: number, e: React.MouseEvent) => {
        e.preventDefault();
        const url = `${baseUrl}${buildQueryString({...queryParams, page})}`;
        
        // Use context if available, otherwise use React.useTransition directly
        if (transitionContext) {
            const { startTransition, setIsPaginationTransition } = transitionContext;
            setIsPaginationTransition(true);
            startTransition(() => {
                router.push(url);
            });
        } else {
            reactStartTransition(() => {
                router.push(url);
            });
        }
    };

    const getItemProps = (page: number) => ({
        component: Link,
        href: `${baseUrl}${buildQueryString({...queryParams, page})}`,
        onClick: (e: React.MouseEvent) => navigateToPage(page, e),
    });

    const getControlProps = (control: 'first' | 'previous' | 'last' | 'next') => {
        if (control === 'next') {
            const nextPage = pagination.page === pagination.total ? pagination.page : pagination.page + 1;
            return {
                component: Link,
                href: `${baseUrl}${buildQueryString({...queryParams, page: nextPage})}`,
                onClick: (e: React.MouseEvent) => navigateToPage(nextPage, e),
            };
        }

        if (control === 'previous') {
            const prevPage = pagination.page === 1 ? pagination.page : pagination.page - 1;
            return {
                component: Link,
                href: `${baseUrl}${buildQueryString({...queryParams, page: prevPage})}`,
                onClick: (e: React.MouseEvent) => navigateToPage(prevPage, e),
            };
        }

        return {};
    };

    return (
        <Pagination
            total={pagination.total}
            value={pagination.page}
            getItemProps={getItemProps}
            getControlProps={getControlProps}
        />
    );
};

export {NextPagination};
