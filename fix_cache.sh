#!/bin/bash

# 修复dao包中的缓存调用，添加nil检查
echo "🔧 修复dao包中的缓存调用..."

# 修复user.go中的缓存调用
echo "修复 user.go..."
sed -i 's/getRedisCache()\.SetUser/getRedisCache().SetUser/g' dao/user.go
sed -i 's/getRedisCache()\.GetUserByName/getRedisCache().GetUserByName/g' dao/user.go
sed -i 's/getRedisCache()\.GetUser/getRedisCache().GetUser/g' dao/user.go
sed -i 's/getRedisCache()\.DeleteUser/getRedisCache().DeleteUser/g' dao/user.go
sed -i 's/getRedisCache()\.DeleteFriendsList/getRedisCache().DeleteFriendsList/g' dao/user.go

# 修复relation.go中的缓存调用
echo "修复 relation.go..."
sed -i 's/getRedisCache()\.GetFriendsList/getRedisCache().GetFriendsList/g' dao/relation.go
sed -i 's/getRedisCache()\.SetFriendsList/getRedisCache().SetFriendsList/g' dao/relation.go
sed -i 's/getRedisCache()\.DeleteFriendsList/getRedisCache().DeleteFriendsList/g' dao/relation.go

echo "✅ 修复完成！"
