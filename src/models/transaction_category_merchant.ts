export interface TransactionCategoryMerchantInfoResponse {
    readonly merchant: string;
    readonly categoryId: string;
}

export interface TransactionCategoryMerchantAddRequest {
    readonly categoryId: string;
    readonly merchant: string;
}

export interface TransactionCategoryMerchantDeleteRequest {
    readonly merchant: string;
}
