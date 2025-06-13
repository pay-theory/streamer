# JWT Code Consolidation Proposal

## Current State Analysis

### JWT Usage Patterns
- **Connect Handler**: ✅ Uses JWT validation during WebSocket handshake
- **Disconnect Handler**: ❌ No JWT needed (authenticates via connection lookup)
- **Router Handler**: ❌ No JWT needed (authenticates via connection lookup)
- **Processor Handler**: ❌ No JWT needed (authenticates via connection lookup)

### Code Duplication
Both original and Lift implementations share:
- `auth.go` (151 lines) - JWT verification logic
- `common.go` (24 lines) - Configuration and utilities
- Same `Claims` struct, `JWTVerifier` type, validation logic

## Consolidation Opportunities

### 1. Move JWT to Shared Library
**Current:**
```
lambda/connect/auth.go (151 lines)
lambda/connect/common.go (24 lines)
```

**Proposed:**
```
lambda/shared/jwt/
├── verifier.go (JWT verification logic)
├── claims.go (Claims struct and validation)
└── config.go (JWT configuration)
```

**Benefits:**
- ✅ **Single source of truth** for JWT logic
- ✅ **Reusable** across any future handlers that need JWT
- ✅ **Easier testing** with dedicated JWT test suite
- ✅ **Better organization** - JWT concerns separated

### 2. Simplified JWT Interface
**Current Interface:**
```go
type JWTVerifier struct {
    publicKey *rsa.PublicKey
    issuer    string
}

func NewJWTVerifier(publicKeyPEM, issuer string) (*JWTVerifier, error)
func (v *JWTVerifier) Verify(tokenString string) (*Claims, error)
```

**Proposed Simplified Interface:**
```go
// Simple function-based approach
func VerifyJWT(token, publicKeyPEM, issuer string) (*Claims, error)

// Or singleton pattern
var DefaultVerifier *JWTVerifier
func InitJWT(publicKeyPEM, issuer string) error
func VerifyToken(token string) (*Claims, error)
```

### 3. Unified Configuration
**Current:** Each handler has its own `HandlerConfig`
**Proposed:** Shared configuration in `lambda/shared/config/`

## Implementation Plan

### Phase 1: Extract to Shared Library
```bash
# Create shared JWT package
mkdir -p lambda/shared/jwt

# Move and refactor JWT code
mv lambda/connect/auth.go lambda/shared/jwt/verifier.go
# Update imports in both implementations
```

### Phase 2: Simplify Interface
```go
// lambda/shared/jwt/jwt.go
package jwt

import (
    "github.com/pay-theory/streamer/lambda/shared/config"
)

// Simple verification function
func VerifyToken(token string, cfg *config.JWTConfig) (*Claims, error) {
    // Simplified implementation
}
```

### Phase 3: Update Handlers
```go
// Both handler.go and handler_lift.go
import "github.com/pay-theory/streamer/lambda/shared/jwt"

// Simplified usage
claims, err := jwt.VerifyToken(token, h.config.JWT)
```

## Expected Code Reduction

### Before Consolidation:
```
lambda/connect/auth.go:     151 lines
lambda/connect/common.go:    24 lines
Total per handler:          175 lines
```

### After Consolidation:
```
lambda/shared/jwt/:          ~100 lines (simplified)
Per handler import:           ~5 lines
Net reduction per handler:   ~70 lines
```

### Total Savings:
- **Connect handler**: ~70 lines saved
- **Future handlers**: ~175 lines saved each (no need to duplicate JWT)
- **Maintenance**: Single place to update JWT logic

## Additional Benefits

### 1. **Better Testing**
- Dedicated JWT test suite in `lambda/shared/jwt/`
- Easier to test edge cases and security scenarios
- Shared test utilities for JWT token generation

### 2. **Security Improvements**
- Single place to implement security updates
- Easier to audit JWT implementation
- Consistent validation across all handlers

### 3. **Developer Experience**
- Clear separation of concerns
- Easier onboarding for new developers
- Consistent JWT patterns across codebase

## Recommendation

**Proceed with JWT consolidation** for these reasons:

1. **Immediate Value**: ~70 lines reduction in Connect handler
2. **Future Value**: ~175 lines saved for each new handler needing JWT
3. **Quality**: Better organization, testing, and maintainability
4. **Security**: Single source of truth for JWT validation

This consolidation would provide **real code reduction** while improving architecture quality.

## Implementation Priority

**High Priority** - This is a clear win:
- ✅ Reduces duplication
- ✅ Improves maintainability  
- ✅ Provides immediate and future value
- ✅ Low risk (just moving existing working code)

Would you like me to implement this JWT consolidation? 