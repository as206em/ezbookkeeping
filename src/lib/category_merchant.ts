import { CategoryType } from '@/core/category.ts';
import type { TransactionCategory } from '@/models/transaction_category.ts';

export const CATEGORY_MERCHANT_MAX_LENGTH: number = 64;

// Bank SMS transactions store their comment as "Merchant - remark", so the merchant is the part before the first " - "
const MERCHANT_COMMENT_SEPARATOR: string = ' - ';

export function normalizeMerchantName(merchant: string): string {
    return merchant.trim().split(/\s+/).filter(part => !!part).join(' ');
}

export function getMerchantKey(merchant: string): string {
    return normalizeMerchantName(merchant).toLowerCase();
}

export function getMerchantFromTransactionComment(comment: string | undefined | null): string {
    if (!comment) {
        return '';
    }

    const separatorIndex = comment.indexOf(MERCHANT_COMMENT_SEPARATOR);
    const merchant = normalizeMerchantName(separatorIndex >= 0 ? comment.substring(0, separatorIndex) : comment);

    return merchant.length <= CATEGORY_MERCHANT_MAX_LENGTH ? merchant : '';
}

export function canCategoryHaveMerchants(category: TransactionCategory | undefined | null): boolean {
    return !!category && category.parentId !== '0' && (category.type === CategoryType.Expense || category.type === CategoryType.Income);
}

// Returns the merchant worth remembering after a transaction's category changes, or an empty string when there is none
export function getMerchantToRemember(comment: string | undefined | null, oldCategoryId: string, newCategory: TransactionCategory | undefined | null, getCategoryIdByMerchant: (merchant: string) => string | undefined): string {
    if (!newCategory || oldCategoryId === newCategory.id || !canCategoryHaveMerchants(newCategory)) {
        return '';
    }

    const merchant = getMerchantFromTransactionComment(comment);

    if (!merchant || getCategoryIdByMerchant(merchant) === newCategory.id) {
        return '';
    }

    return merchant;
}
