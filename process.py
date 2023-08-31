
import pandas as pd
import numpy as np

# 读取日志文件
with open("ProposerConsensus.log", "r", encoding="utf-8") as f:
    lines = f.readlines()

# 解析数据
data = []
for line in lines:
    parts = line.strip().split(",")
    row = {}
    for part in parts:
        print("Processing part:", part)
        key, value = part.split(":")
        if value[-2:] == "ms":
            row[key] = float(value[:-2]) / 1000  # 转换毫秒为秒
        else:
            row[key] = float(value[:-1])  # 秒，去掉最后的 's'
    data.append(row)

# 创建数据框
df = pd.DataFrame(data)

# 定义正态分布差值参数
mean = 0
std_dev = 0.1

# 为每个数据添加正态分布差值
for column in df.columns:
    noise = np.random.normal(mean, std_dev, len(df))
    df[column] += noise

# 将结果写入Excel文件
df.to_excel("ProposerConsensus.xlsx", index=False)
