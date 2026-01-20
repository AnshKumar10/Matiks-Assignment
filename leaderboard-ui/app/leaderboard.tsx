import React, { useEffect, useMemo, useState, useRef } from "react";
import {
  Text,
  StyleSheet,
  TextInput,
  View,
  RefreshControl,
  TouchableOpacity,
  FlatList,
} from "react-native";
import { Search } from "lucide-react";
import { getLeaderboard, searchUsers, updateRandomRating } from "./api";
import { debounce } from "./utils";
import { LeaderboardUser } from "./types";

export default function LeaderboardScreen() {
  const [leaderboard, setLeaderboard] = useState<LeaderboardUser[]>([]);
  const [searchResults, setSearchResults] = useState<LeaderboardUser[]>([]);
  const [searchQuery, setSearchQuery] = useState<string>("");
  const [refreshing, setRefreshing] = useState<boolean>(false);
  const flatListRef = useRef<FlatList>(null);

  const loadLeaderboard = async () => {
    try {
      setRefreshing(true);
      const data = await getLeaderboard();
      setLeaderboard(data);
    } catch (e) {
      console.error(e);
    } finally {
      setRefreshing(false);
    }
  };

  useEffect(() => {
    loadLeaderboard();
  }, []);

  const handleSearch = useMemo(
    () =>
      debounce(async (text) => {
        if (text.length > 0) {
          const results = await searchUsers(text);
          setSearchResults(results);
        } else {
          setSearchResults([]);
        }
      }, 500),
    [],
  );

  const onChangeSearch = (text: string): void => {
    setSearchQuery(text);
    handleSearch(text);
  };

  const handleUpdateRandom = async () => {
    flatListRef.current?.scrollToOffset({ offset: 0, animated: true });
    await updateRandomRating();
    setSearchResults([]);
    setSearchQuery("");
    loadLeaderboard();
  };

  const displayData = searchQuery.length > 0 ? searchResults : leaderboard;

  return (
    <View style={styles.page}>
      <View style={styles.container}>
        <View style={styles.header}>
          <View>
            <Text style={styles.title}>LEADERBOARD</Text>
            <Text style={styles.subtitle}>Correctness, Scale and Clarity</Text>
          </View>
          <TouchableOpacity
            style={styles.randomButton}
            onPress={handleUpdateRandom}
          >
            <Text style={styles.randomButtonText}>Simulate Update Rating</Text>
          </TouchableOpacity>
        </View>

        <View style={styles.searchWrapper}>
          <View style={styles.searchIcon}>
            <Search size={20} color="#6b7280" />
          </View>
          <TextInput
            value={searchQuery}
            onChangeText={onChangeSearch}
            placeholder="Search player..."
            placeholderTextColor="#4b5563"
            style={styles.searchInput}
          />
        </View>

        <FlatList
          ref={flatListRef}
          style={styles.tableScroll}
          contentContainerStyle={styles.table}
          data={displayData}
          keyExtractor={(item: LeaderboardUser) => item.user_id}
          refreshControl={
            <RefreshControl
              refreshing={refreshing}
              onRefresh={loadLeaderboard}
            />
          }
          renderItem={({ item }) => (
            <View style={styles.row}>
              <Text
                style={[
                  styles.rank,
                  item.rank === 1 && styles.rankFirst,
                  item.rank === 2 && styles.rankSecond,
                  item.rank === 3 && styles.rankThird,
                ]}
              >
                [{item.rank}]
              </Text>

              <Text
                style={[styles.username, item.rank <= 3 && styles.usernameTop]}
              >
                {item.username}
              </Text>

              <View style={styles.ratingWrapper}>
                <View style={styles.dot} />
                <Text style={styles.rating}>{item.rating}</Text>
              </View>
            </View>
          )}
          ListEmptyComponent={
            !refreshing ? (
              <View style={styles.empty}>
                <Text style={styles.emptyText}>No players found</Text>
              </View>
            ) : null
          }
        />
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  page: {
    flex: 1,
    backgroundColor: "#000",
    paddingVertical: 32,
    paddingHorizontal: 16,
    fontFamily: "monospace",
  },
  container: {
    maxWidth: 1100,
    alignSelf: "center",
    width: "100%",
    flex: 1,
  },
  header: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    marginBottom: 16,
  },
  title: {
    fontSize: 36,
    fontWeight: "700",
    fontFamily: "monospace",
    color: "#00ff41",
    marginBottom: 8,
    letterSpacing: 2,
  },
  subtitle: {
    color: "#9ca3af",
    fontFamily: "monospace",
    fontSize: 16,
    textAlign: "center",
  },
  randomButton: {
    backgroundColor: "#00ff41",
    paddingVertical: 12,
    paddingHorizontal: 24,
    borderRadius: 8,
    marginBottom: 20,
    alignSelf: "center",
  },
  randomButtonText: {
    color: "#000",
    fontWeight: "700",
    fontFamily: "monospace",
  },
  searchWrapper: {
    marginBottom: 16,
    alignSelf: "center",
    position: "relative",
    width: "100%",
  },
  searchIcon: {
    position: "absolute",
    left: 12,
    top: 12,
    zIndex: 2,
  },
  searchInput: {
    borderWidth: 1,
    borderColor: "#1f2933",
    borderRadius: 8,
    paddingVertical: 12,
    paddingLeft: 40,
    paddingRight: 12,
    color: "#fff",
    backgroundColor: "transparent",
  },
  tableScroll: {
    flex: 1,
    marginTop: 8,
  },
  table: {
    borderTopWidth: 1,
    borderTopColor: "#1f2933",
    paddingRight: 12,
  },
  row: {
    flexDirection: "row",
    alignItems: "center",
    borderBottomWidth: 1,
    borderBottomColor: "#111827",
    paddingVertical: 16,
  },
  rank: {
    width: 60,
    color: "#6b7280",
    fontSize: 13,
    fontFamily: "monospace",
  },
  rankFirst: {
    color: "#00ff41",
    fontWeight: "700",
  },
  rankSecond: {
    color: "#d1d5db",
    fontWeight: "700",
  },
  rankThird: {
    color: "#9ca3af",
    fontWeight: "700",
  },
  username: {
    flex: 1,
    color: "#d1d5db",
    fontFamily: "monospace",
  },
  usernameTop: {
    color: "#ffffff",
  },
  ratingWrapper: {
    flexDirection: "row",
    alignItems: "center",
  },
  rating: {
    color: "#fff",
    fontFamily: "monospace",
    marginRight: 8,
  },
  dot: {
    width: 8,
    height: 8,
    margin: 10,
    borderRadius: 4,
    backgroundColor: "#00d4ff",
  },
  empty: {
    paddingVertical: 64,
    alignItems: "center",
  },
  emptyText: {
    color: "#6b7280",
    fontFamily: "monospace",
  },
});
