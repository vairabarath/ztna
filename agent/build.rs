fn main() {
    println!("cargo:rerun-if-changed=../proto/ztna/controlplane/v1/controlplane.proto");
    println!("cargo:rerun-if-changed=../proto/ztna/dataplane/v1/dataplane.proto");

    tonic_build::configure()
        .compile_protos(
            &[
                "../proto/ztna/controlplane/v1/controlplane.proto",
                "../proto/ztna/dataplane/v1/dataplane.proto",
            ],
            &["../proto"],
        )
        .expect("compile protobuf definitions");
}
